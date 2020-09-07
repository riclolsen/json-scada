# libplctag .NET

This wrapper project is still in alpha. If you have problems, please refer to the [project's contact information](https://github.com/libplctag/libplctag#contact).

This repository contains two .NET wrappers for [libplctag](https://github.com/libplctag/libplctag) that are published to Nuget.org:
* [libplctag](https://www.nuget.org/packages/libplctag/) ![Nuget version](https://img.shields.io/nuget/vpre/libplctag) ![Nuget](https://img.shields.io/nuget/dt/libplctag)
* [libplctag.NativeImport](https://www.nuget.org/packages/libplctag.NativeImport/) ![Nuget version](https://img.shields.io/nuget/vpre/libplctag.NativeImport) ![Nuget](https://img.shields.io/nuget/dt/libplctag.NativeImport)

## Goals

* Package the libplctag functionality in a way that is convenient to use in .NET applications (i.e. publish on nuget.org).
* Be cross-platform: It should support any platform that libplctag can be built for, and supports .NET Standard 2.0

## libplctag

This is the package intended for use by application developers. It provides strongly-typed and convenient access to Tags.

For usage, see the examples in the example projects:

* [C#](https://github.com/libplctag/libplctag.NET/tree/master/src/Examples/CSharp%20DotNetCore)
* [VB.NET](https://github.com/libplctag/libplctag.NET/blob/master/src/Examples/VB.NET%20DotNetCore/Program.vb)


## libplctag.NativeImport

libplctag.NativeImport provides low-level (raw) access to the native libplctag library. This package does not provide an API which is friendly to application developers. Rather, it provides an API which handles platform and configuration issues, upon which a friendlier API can be built.

An example of it's usage can be found [here](https://github.com/libplctag/libplctag.NET/blob/master/src/Examples/CSharp%20DotNetCore/NativeImportExample.cs).
