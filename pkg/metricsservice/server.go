/*
Copyright 2022 The KEDA Authors

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

package metricsservice

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"k8s.io/metrics/pkg/apis/external_metrics/v1beta1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kedacore/keda/v2/pkg/metricsservice/api"
	"github.com/kedacore/keda/v2/pkg/metricsservice/utils"
	"github.com/kedacore/keda/v2/pkg/scaling"
)

var log = logf.Log.WithName("grpc_server")

type GrpcServer struct {
	server        *grpc.Server
	healthServer  *health.Server
	errChan       chan error
	address       string
	certDir       string
	certsReady    chan struct{}
	scalerHandler *scaling.ScaleHandler
	api.UnimplementedMetricsServiceServer
}

// GetMetrics returns metrics values in form of ExternalMetricValueList for specified ScaledObject reference
func (s *GrpcServer) GetMetrics(ctx context.Context, in *api.ScaledObjectRef) (*v1beta1.ExternalMetricValueList, error) {
	v1beta1ExtMetrics := &v1beta1.ExternalMetricValueList{}
	extMetrics, err := (*s.scalerHandler).GetScaledObjectMetrics(ctx, in.Name, in.Namespace, in.MetricName)
	if err != nil {
		return v1beta1ExtMetrics, fmt.Errorf("error when getting metric values %w", err)
	}

	err = v1beta1.Convert_external_metrics_ExternalMetricValueList_To_v1beta1_ExternalMetricValueList(extMetrics, v1beta1ExtMetrics, nil)
	if err != nil {
		return v1beta1ExtMetrics, fmt.Errorf("error when converting metric values %w", err)
	}

	log.V(1).WithValues("scaledObjectName", in.Name, "scaledObjectNamespace", in.Namespace, "metrics", v1beta1ExtMetrics).Info("Providing metrics")

	return v1beta1ExtMetrics, nil
}

// NewGrpcServer creates a new instance of GrpcServer
func NewGrpcServer(scaleHandler *scaling.ScaleHandler, address, certDir string, certsReady chan struct{}) GrpcServer {
	return GrpcServer{
		address:       address,
		scalerHandler: scaleHandler,
		certDir:       certDir,
		certsReady:    certsReady,
	}
}

func (s *GrpcServer) startServer() error {
	lis, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	if err := s.server.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// StartGrpcServer starts the grpc server in non-serving mode
func (s *GrpcServer) StartGrpcServer(ctx context.Context) error {
	<-s.certsReady
	if s.server == nil {
		creds, err := utils.LoadGrpcTLSCredentials(s.certDir, true)
		if err != nil {
			return err
		}
		s.server = grpc.NewServer(grpc.Creds(creds))
		api.RegisterMetricsServiceServer(s.server, s)

		s.healthServer = health.NewServer()
		s.healthServer.SetServingStatus(api.MetricsService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		grpc_health_v1.RegisterHealthServer(s.server, s.healthServer)
	}

	s.errChan = make(chan error)

	go func() {
		log.Info("Starting Metrics Service gRPC Server", "address", s.address)
		if err := s.startServer(); err != nil {
			err := fmt.Errorf("unable to start Metrics Service gRPC server on address %s, error: %w", s.address, err)
			log.Error(err, "error starting Metrics Service gRPC server")
			s.errChan <- err
		}
	}()

	return nil
}

// Start sets the status of the GRPC server to SERVING after the leader election lock is acquired
func (s *GrpcServer) Start(ctx context.Context) error {
	s.healthServer.SetServingStatus(api.MetricsService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)

	select {
	case err := <-s.errChan:
		return err
	case <-ctx.Done():
		return nil
	}
}

// NeedLeaderElection is needed to implement LeaderElectionRunnable interface
// of controller-runtime. This assures that the component is started/stoped
// when this particular instance is selected/deselected as a leader.
func (s *GrpcServer) NeedLeaderElection() bool {
	return true
}
