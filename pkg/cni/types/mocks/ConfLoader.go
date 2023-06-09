// Code generated by mockery v2.27.1. DO NOT EDIT.

package mocks

import (
	types "github.com/Mellanox/nvidia-k8s-ipam/pkg/cni/types"
	mock "github.com/stretchr/testify/mock"
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

// LoadConf provides a mock function with given fields: bytes
func (_m *ConfLoader) LoadConf(bytes []byte) (*types.NetConf, error) {
	ret := _m.Called(bytes)

	var r0 *types.NetConf
	var r1 error
	if rf, ok := ret.Get(0).(func([]byte) (*types.NetConf, error)); ok {
		return rf(bytes)
	}
	if rf, ok := ret.Get(0).(func([]byte) *types.NetConf); ok {
		r0 = rf(bytes)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.NetConf)
		}
	}

	if rf, ok := ret.Get(1).(func([]byte) error); ok {
		r1 = rf(bytes)
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
//   - bytes []byte
func (_e *ConfLoader_Expecter) LoadConf(bytes interface{}) *ConfLoader_LoadConf_Call {
	return &ConfLoader_LoadConf_Call{Call: _e.mock.On("LoadConf", bytes)}
}

func (_c *ConfLoader_LoadConf_Call) Run(run func(bytes []byte)) *ConfLoader_LoadConf_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]byte))
	})
	return _c
}

func (_c *ConfLoader_LoadConf_Call) Return(_a0 *types.NetConf, _a1 error) *ConfLoader_LoadConf_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ConfLoader_LoadConf_Call) RunAndReturn(run func([]byte) (*types.NetConf, error)) *ConfLoader_LoadConf_Call {
	_c.Call.Return(run)
	return _c
}

type mockConstructorTestingTNewConfLoader interface {
	mock.TestingT
	Cleanup(func())
}

// NewConfLoader creates a new instance of ConfLoader. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewConfLoader(t mockConstructorTestingTNewConfLoader) *ConfLoader {
	mock := &ConfLoader{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
