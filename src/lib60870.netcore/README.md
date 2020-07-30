# Lib60870.NET Based Drivers.

Here you have the code for the lib60870.NET based drivers.
Here is the official repo for the library.
https://github.com/mz-automation/lib60870.NET

The code is intended to be compiled with the DOTNET CORE 3.1 SDK on any supported OS/CPU platform, including Linux, Windows, x86/64, ARM.

This library is pretty capable (thanks MZ Automation). It was used to develop the following drivers.

* IEC60870-5-101 Client (serial and TCP).
* IEC60870-5-101 Server (serial and TCP).
* IEC60870-5-104 Client.
* IEC60870-5-104 Server.

Protocol-side TLS connections are not implemented at this moment. TLS is supported on the database connection side.

All the four drivers support multiple connections on multiple instances of executables on the same or on multiple computers. This distributed processing architecture enables to connect to unlimited devices with an incredible throughput that depends only on the database server (or cluster) capability on handling the incoming data. The  MongoDB database server can scale horizontally via sharding, so the capacity of data acquisition can be really huge.

