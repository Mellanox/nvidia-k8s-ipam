// Code generated by mockery v2.49.1. DO NOT EDIT.

package mocks

import (
	skel "github.com/containernetworking/cni/pkg/skel"
	mock "github.com/stretchr/testify/mock"

	types "github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
)

// ConfLoader is an autogenerated mock type for the ConfLoader type
type ConfLoader struct {
	mock.Mock
}

type ConfLoader_Expecter struct {
	mock *mock.Mock
}

func (_m *ConfLoader) EXPECT() *ConfLoader_Expecter {
	return &ConfLoader_Expecter{mock: &_m.Mock}
}

// LoadConf provides a mock function with given fields: args
func (_m *ConfLoader) LoadConf(args *skel.CmdArgs) (*types.NetConf, error) {
	ret := _m.Called(args)

	if len(ret) == 0 {
		panic("no return value specified for LoadConf")
	}

	var r0 *types.NetConf
	var r1 error
	if rf, ok := ret.Get(0).(func(*skel.CmdArgs) (*types.NetConf, error)); ok {
		return rf(args)
	}
	if rf, ok := ret.Get(0).(func(*skel.CmdArgs) *types.NetConf); ok {
		r0 = rf(args)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.NetConf)
		}
	}

	if rf, ok := ret.Get(1).(func(*skel.CmdArgs) error); ok {
		r1 = rf(args)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ConfLoader_LoadConf_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'LoadConf'
type ConfLoader_LoadConf_Call struct {
	*mock.Call
}

// LoadConf is a helper method to define mock.On call
//   - args *skel.CmdArgs
func (_e *ConfLoader_Expecter) LoadConf(args interface{}) *ConfLoader_LoadConf_Call {
	return &ConfLoader_LoadConf_Call{Call: _e.mock.On("LoadConf", args)}
}

func (_c *ConfLoader_LoadConf_Call) Run(run func(args *skel.CmdArgs)) *ConfLoader_LoadConf_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*skel.CmdArgs))
	})
	return _c
}

func (_c *ConfLoader_LoadConf_Call) Return(_a0 *types.NetConf, _a1 error) *ConfLoader_LoadConf_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ConfLoader_LoadConf_Call) RunAndReturn(run func(*skel.CmdArgs) (*types.NetConf, error)) *ConfLoader_LoadConf_Call {
	_c.Call.Return(run)
	return _c
}

// NewConfLoader creates a new instance of ConfLoader. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewConfLoader(t interface {
	mock.TestingT
	Cleanup(func())
}) *ConfLoader {
	mock := &ConfLoader{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
