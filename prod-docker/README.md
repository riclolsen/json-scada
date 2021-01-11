# Docker Demo

This is a full executable docker demo for the system.

It includes 

* MongoDB Community as the core database server.
* PostgreSQL/TimescaleDB for time series historian.
* Grafana for dashboards.
* IEC 60870-5-104 Client that connects to the online demo for data acquisition.
* IEC 60870-5-104 Server listening on the localhost.
* Calculations processor.
* Change stream realtime data processor.
* A Node/Express webserver app for user interface.

To run this demo, a docker runtime is needed with docker-compose command available. Git is also needed to extract the repository.

It can run on any Linux x64 or Windows 10 x64 (use Docker/WSL2 on Windows 10 version 2004 for best performance on this platform).

Clone the whole repository on the host computer. 

	git clone https://github.com/riclolsen/json-scada.git  --config core.autocrlf=input

Go to the compile-docker folder to create binaries.

	cd json-scada/compile-docker
	docker-compose up

Wait until the compilation process finishes.

Go to the demo-docker folder and run the system.
	
	cd ../demo-docker
	docker-compose up

Wait until images are pulled, the databases are seeded and the protocol communication begins.

Access http://localhost:8080 on Chrome or Firefox.

To access Grafana the credentials are user="admin" and password="jsonscada".

This demo will connect to the online demo using IEC60870-5-104. 

Online demo: http://vmi233205.contaboserver.net:8080/.

When the system docker demo and the online demo are connected, both systems will show the same data and will reflect identically the results of commands.

The MongoDB and PostgreSQL are configured for unauthenticated access, server ports are not exported to the main host (this can be changed in the _docker-compose.yaml_ file).

The docker demo also provides an IEC60870-5-104 server port (127.0.0.1:2404, originator address 1) that you can connect to using some IEC60870-5-104 client tool.

Suggestions of free IEC60870-5-104 clients 

* https://github.com/riclolsen/qtester104
* http://the-vinci.com/vinci-software.
* https://www.mz-automation.de/communication-protocols/iec-60870-5-104-test-tool/