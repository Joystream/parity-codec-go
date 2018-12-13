# parity-codec-go

Port of https://github.com/paritytech/parity-codec/ to Go.

Feature parity is almost full, apart from:

* the lack of support for u128 (which are missing in Go);
* one has to manually define OptionX types for Option decoding to properly work.
