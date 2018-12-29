# parity-codec-go

Port of https://github.com/paritytech/parity-codec/ to Go.

There are two versions:

`withreflect`: easier to use, but slower version using Go reflection.

`noreflect`: does not use reflection, so is suitable to be used with https://github.com/aykevl/tinygo/

### TODO

* support ToKeyedVec in noreflect version
* emulate Rust's Result type
* code generation for slice types in noreflect version
* code generation for option types