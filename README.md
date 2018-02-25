= PNG to XNB converter in Go

Similar to my old [PNG to XNB converter](https://github.com/sullerandras/png_to_xnb), but this is in Go so (at least theoretically) it is platform independent and could be executed natively on all major operating systems.

== How to compile

```
go build -o png_to_xnb main.go
```

== How to use

Compile it, then you can convert an image to png like this:

```
./png_to_xnb input.png output.xnb

```

To convert all PNGs in a directory and write the XNBs to an other directory:

```
./png_to_xnb input_folder output_folder
```

== License

MIT, see LICENSE file