# Mac Install

I test using a Macbook Air M1 using Rosetta. Install on Intel Macs should also work. 
Native M1 install should be available some time after Homebrew matures for Apple ARM M1 platform.

## Thoughts on Apple ARM M1 platform

{json:scada} is fully portable to platforms where you can run MongoDB, Postgresql, Node.js, Go and Dotnet. So Macs can also be targeted.
I think the M1 will be the perfect platform for running {json:scada} servers and clients for big and small systems.
For local control, the Mac Mini is ideal, it is powerful and power efficient, is has only one fan. It will  be a nice option for harsh environments like substations and power plants.
For control centers the Mac Mini can be a nice and cheap solution for either a {json:scada} server or a client for operators.
  
## Install Homebrew

Mac M1 Using Rosetta

    /usr/sbin/softwareupdate --install-rosetta

Follo instructions from here

* https://dev.to/redhoodjt1988/how-to-setup-development-environment-on-new-macbook-pro-m1-3kh
    
Proceed installig requisites using Homebrew.

    brew install node
    brew install golang
    brew install dotnet-sdk
    brew tap mongodb/brew
    brew install mongodb-community
    brew tap timescale/tap
    brew install timescaledb
    # If found some error use this
    ln -s /usr/local/Cellar/postgresql@12/12.5/include/postgresql/server /usr/local/Cellar/postgresql@12/12.5/include/server
    brew install timescaledb
    ln -s /usr/local/var/postgresql@12 /usr/local/var/postgres
    export PATH=$PATH:/usr/local/Cellar/postgresql@12/12.5/bin

    # Post-install to move files to appropriate place
    /usr/local/bin/timescaledb_move.sh

    timescaledb-tune

Add replica set option to mongodb.conf

    nano /usr/local/etc/mongod.conf
    # add this
    replication:
        replSetName: "rs1"

    brew services restart postgresql@12
    brew services restart mongodb-community

Check for services running

    brew services

Apply build instructions from here

* * [Install Guide](../docs/install.md)

