// Code generated by encore. DO NOT EDIT.
//
// The contents of this file are generated from the structs used in
// conjunction with Encore's `config.Load[T]()` function. This file
// automatically be regenerated if the data types within the struct
// are changed.
//
// For more information about this file, see:
// https://encore.dev/docs/develop/config
package svc

#ServerOption: {
	Option:    int64
	Disabled?: bool // True if this is disabled
}
HTTP:          #ServerOption
a_n_o_t_h_e_r: #ServerOption
TCP?:          #ServerOption
GRPC?:         #ServerOption
