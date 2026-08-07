// Package metalmock provides a configurable mock for the metal-stack apiv2 connect
// client (github.com/metal-stack/api) that can be injected into the controllers via
// the ControllerConfig. It uses a connect client interceptor to serve the machine,
// network and image service calls and delegates each call to a programmable handler.
package test

import (
	"context"
	"sync"
	"testing"

	"connectrpc.com/connect"
	apiv2client "github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	"github.com/metal-stack/api/go/metalstack/api/v2/apiv2connect"
)

// Handler signatures for the service methods used by the controllers.

type (
	MachineGetFn    func(context.Context, *apiv2.MachineServiceGetRequest) (*apiv2.MachineServiceGetResponse, error)
	MachineCreateFn func(context.Context, *apiv2.MachineServiceCreateRequest) (*apiv2.MachineServiceCreateResponse, error)
	MachineUpdateFn func(context.Context, *apiv2.MachineServiceUpdateRequest) (*apiv2.MachineServiceUpdateResponse, error)
	MachineListFn   func(context.Context, *apiv2.MachineServiceListRequest) (*apiv2.MachineServiceListResponse, error)
	MachineDeleteFn func(context.Context, *apiv2.MachineServiceDeleteRequest) (*apiv2.MachineServiceDeleteResponse, error)

	NetworkGetFn func(context.Context, *apiv2.NetworkServiceGetRequest) (*apiv2.NetworkServiceGetResponse, error)

	ImageLatestFn func(context.Context, *apiv2.ImageServiceLatestRequest) (*apiv2.ImageServiceLatestResponse, error)
)

// Client is a configurable mock apiv2 client implemented as a connect client
// interceptor. Set the various handler fields before using it. Unset handlers
// answer with connect.CodeUnimplemented. The handler fields may be reconfigured
// at any time (e.g. between calls in an integration test).
type Client struct {
	t *testing.T

	mu sync.RWMutex

	OnMachineGet    MachineGetFn
	OnMachineCreate MachineCreateFn
	OnMachineUpdate MachineUpdateFn
	OnMachineList   MachineListFn
	OnMachineDelete MachineDeleteFn

	OnNetworkGet NetworkGetFn

	OnImageLatest ImageLatestFn

	apiv2 apiv2client.Client
}

// New creates a mock apiv2 client backed by a connect client interceptor.
func New(t *testing.T) *Client {
	return &Client{t: t}
}

// Client returns the apiv2 client that can be passed to the ControllerConfig.
// It is constructed lazily so that the returned client is always wired with this
// mock as its interceptor.
func (m *Client) Client() apiv2client.Client {
	m.mu.RLock()
	c := m.apiv2
	m.mu.RUnlock()
	if c != nil {
		return c
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.apiv2 != nil {
		return m.apiv2
	}

	c, err := apiv2client.New(&apiv2client.DialConfig{
		BaseURL:      "http://localhost",
		Interceptors: []connect.Interceptor{m},
	})
	if err != nil {
		m.t.Fatalf("unable to create mock apiv2 client: %v", err)
	}
	m.apiv2 = c
	return c
}

// WrapUnary implements connect.UnaryInterceptorClient. It dispatches the call to
// the configured handler based on the connect procedure, or returns
// connect.CodeUnimplemented when no handler is set for the call.
func (m *Client) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		m.mu.RLock()
		defer m.mu.RUnlock()

		var (
			resp connect.AnyResponse
			err  error
		)
		switch req.Spec().Procedure {
		case apiv2connect.MachineServiceGetProcedure:
			var r *apiv2.MachineServiceGetResponse
			if m.OnMachineGet != nil {
				r, err = m.OnMachineGet(ctx, req.Any().(*apiv2.MachineServiceGetRequest))
			}
			if r != nil {
				resp = connect.NewResponse(r)
			}
		case apiv2connect.MachineServiceCreateProcedure:
			var r *apiv2.MachineServiceCreateResponse
			if m.OnMachineCreate != nil {
				r, err = m.OnMachineCreate(ctx, req.Any().(*apiv2.MachineServiceCreateRequest))
			}
			if r != nil {
				resp = connect.NewResponse(r)
			}
		case apiv2connect.MachineServiceUpdateProcedure:
			var r *apiv2.MachineServiceUpdateResponse
			if m.OnMachineUpdate != nil {
				r, err = m.OnMachineUpdate(ctx, req.Any().(*apiv2.MachineServiceUpdateRequest))
			}
			if r != nil {
				resp = connect.NewResponse(r)
			}
		case apiv2connect.MachineServiceListProcedure:
			var r *apiv2.MachineServiceListResponse
			if m.OnMachineList != nil {
				r, err = m.OnMachineList(ctx, req.Any().(*apiv2.MachineServiceListRequest))
			}
			if r != nil {
				resp = connect.NewResponse(r)
			}
		case apiv2connect.MachineServiceDeleteProcedure:
			var r *apiv2.MachineServiceDeleteResponse
			if m.OnMachineDelete != nil {
				r, err = m.OnMachineDelete(ctx, req.Any().(*apiv2.MachineServiceDeleteRequest))
			}
			if r != nil {
				resp = connect.NewResponse(r)
			}
		case apiv2connect.NetworkServiceGetProcedure:
			var r *apiv2.NetworkServiceGetResponse
			if m.OnNetworkGet != nil {
				r, err = m.OnNetworkGet(ctx, req.Any().(*apiv2.NetworkServiceGetRequest))
			}
			if r != nil {
				resp = connect.NewResponse(r)
			}
		case apiv2connect.ImageServiceLatestProcedure:
			var r *apiv2.ImageServiceLatestResponse
			if m.OnImageLatest != nil {
				r, err = m.OnImageLatest(ctx, req.Any().(*apiv2.ImageServiceLatestRequest))
			}
			if r != nil {
				resp = connect.NewResponse(r)
			}
		default:
			return nil, connect.NewError(connect.CodeUnimplemented, nil)
		}

		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, nil)
		}
		return resp, nil
	}
}

// WrapStreamingClient implements connect.UnaryInterceptorClient. The controllers
// only use unary calls, so this is not supported.
func (m *Client) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	m.t.Errorf("metalmock does not support streaming client calls")
	return nil
}

// WrapStreamingHandler implements connect.UnaryInterceptorClient. The controllers
// only use unary calls, so this is not supported.
func (m *Client) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	m.t.Errorf("metalmock does not support streaming handler calls")
	return nil
}
