/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kubeflow/training-operator/pkg/config"
	controller_v1 "github.com/kubeflow/training-operator/pkg/controller.v1"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	mpiv1 "github.com/kubeflow/training-operator/pkg/apis/mpi/v1"
	mxnetv1 "github.com/kubeflow/training-operator/pkg/apis/mxnet/v1"
	pytorchv1 "github.com/kubeflow/training-operator/pkg/apis/pytorch/v1"
	tensorflowv1 "github.com/kubeflow/training-operator/pkg/apis/tensorflow/v1"
	xgboostv1 "github.com/kubeflow/training-operator/pkg/apis/xgboost/v1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(xgboostv1.AddToScheme(scheme))
	utilruntime.Must(pytorchv1.AddToScheme(scheme))
	utilruntime.Must(tensorflowv1.AddToScheme(scheme))
	utilruntime.Must(mxnetv1.AddToScheme(scheme))
	utilruntime.Must(mpiv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var enabledSchemes controller_v1.EnabledSchemes
	var enableGangScheduling bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Var(&enabledSchemes, "enable-scheme", "Enable scheme(s) as --enable-scheme=tfjob --enable-scheme=pytorchjob, case insensitive."+
		" Now supporting TFJob, PyTorchJob, MXNetJob, XGBoostJob. By default, all supported schemes will be enabled.")
	flag.BoolVar(&enableGangScheduling, "enable-gang-scheduling", false, "Set true to enable gang scheduling")

	// PyTorch related flags
	flag.StringVar(&config.Config.PyTorchInitContainerImage, "pytorch-init-container-image",
		config.PyTorchInitContainerImageDefault, "The image for pytorch init container")
	flag.StringVar(&config.Config.PyTorchInitContainerTemplateFile, "pytorch-init-container-template-file",
		config.PyTorchInitContainerTemplateFileDefault, "The template file for pytorch init container")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "1ca428e5.",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// TODO: We need a general manager. all rest reconciler addsToManager
	// Based on the user configuration, we start different controllers
	if enabledSchemes.Empty() {
		enabledSchemes.FillAll()
	}
	for _, s := range enabledSchemes {
		setupFunc, supported := controller_v1.SupportedSchemeReconciler[s]
		if !supported {
			setupLog.Error(fmt.Errorf("cannot find %s in supportedSchemeReconciler", s),
				"scheme not supported", "scheme", s)
			os.Exit(1)
		}
		if err = setupFunc(mgr, enableGangScheduling); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", s)
			os.Exit(1)
		}
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
