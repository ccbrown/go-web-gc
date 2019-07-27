# wasm-go-playground

This is the Go compiler ("gc") compiled for WASM, running in your browser! It can be used to run a simple playground, à la [play.golang.org](https://play.golang.org/) entirely in your browser!

You can try it out here: https://ccbrown.github.io/wasm-go-playground

#### ⚠️ Important ⚠️

* Safari works, but is unbearably slow. **Chrome or Firefox for desktop is highly recommended.**
* Imports other than "runtime" are not supported. Making them work would be straightforward, but I'm not compelled to spend time on it right now since this probably has no practical uses.

## Experimental Playgrounds

One potential use-case for this is making compiler changes easily and freely available for people to try out.

If you're interested in trying out the `try` [proposal](https://github.com/golang/proposal/blob/master/design/32437-try-builtin.md), you can do so here: https://ccbrown.github.io/wasm-go-playground/experimental/try-builtin/

If you're interested in trying out generics, you can do so here: https://ccbrown.github.io/wasm-go-playground/experimental/generics/

## Code

* ./cmd – These are Go commands compiled for WASM. They were all produced by running commands such as `GOOS=js GOARCH=wasm go build .` from the Go source directories.
* ./experimental – This directory contains experimental playgrounds, for testing out modified compilers.
* ./prebuilt – These are prebuilt runtime WASM files. These were produced by copying them from Go's cache after compiling anything for WASM.
* . – The top level directory contains all the static files for the in-browser Go playground. Most of the files are lightly modified copies of bits and pieces from [play.golang.org](https://play.golang.org/). The most substantial work here is in index.html and wasm_exec.js as wasm_exec.js needed a virtual filesystem implementation.
