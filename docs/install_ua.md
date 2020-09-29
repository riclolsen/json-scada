# Встановлення JSON-SCADA

Щоб встановити JSON-SCADA, спочатку потрібно встановити всі вимоги. Немає сенсу відтворювати оригінальні вказівки щодо встановлення для кожного попереднього проекту. Ось посилання та відповідна інформація, характерна для JSON-SCADA.

## Підтримувані апаратні/ОС платформи

* Найбільш сучасні Linux x86-64 біти. Рекомендувати Centos/Redhat 8.2.

* Windows 10 або Server x86-64 біт.

* Linux ARM 32 біти (протестовано принаймні для драйверів протоколів на Raspberry Pi 3/Raspbian OS).

Можливо, він також може працювати на MacOS та Linux ARM-64.

Повна система може працювати на одному комп'ютері x86, але для високої продуктивності та високої доступності у великих системах (> 10 000 тегів) настійно рекомендується наступне обладнання:

* Останні серверні процесори Intel Xeon, AMD Epyc або Threadripper.

* 32 ГБ оперативної пам'яті або більше.

* Ексклюзивні диски даних (XFS, відформатований на Linux) на 512 ГБ + NVMe твердотільних дисках для MongoDB (дзеркальне відображення RAID-1 для високої доступності).

* Ексклюзивні диски даних на 1 ТБ + NVMe SSD для PostgreSQL (RAID-1 дзеркально відображається для високої доступності).

* Кластер набору реплік MongoDB з 3 серверами.

* 2 сервери PostgreSQL з реплікацією.

Для великих систем (наприклад, із більш ніж 200 000 тегів) може знадобитися sharded кластер MongoDB.

## Вимоги до програмного забезпечення

### 1. Сервер MongoDB

Версія 4.2.8 - Нижчі версії не підтримуються і не рекомендуються. Новіші версії можуть працювати, але не перевірялись.

* https://www.mongodb.com/try/download/community
* https://docs.mongodb.com/manual/installation/

Також підтримується хмарний сервіс _MongoDB Atlas_.

Функція _Replica Set_ має бути ввімкнена, навіть коли використовується лише один сервер, оскільки це необхідно для роботи програми Change Streams.

MongoDB підтримує багато архітектур, він дуже гнучкий. Ви можете розгорнути лише на одному сервері, на класичному наборі реплік із 3-х членів або на великому загостреному кластері (із серверами MongoS та конфігурацією).

* https://docs.mongodb.com/manual/core/sharded-cluster-components/

Для ненадійних або відкритих для мереж Інтернету мереж важливо використовувати TLS через з'єднання MongoDB. Зверніться до документів MongoDB, щоб дізнатись, як налаштувати підключення за допомогою сертифікатів.

* https://docs.mongodb.com/manual/tutorial/configure-ssl/

### 2. PostgreSQL / TimescaleDB

Версія PostgreSQL 12. TimescaleDB версія 1.7. Попередні версії можуть працювати, але не рекомендуються. Новіші версії можуть працювати, але не перевірялись.

* https://www.timescale.com/products
* https://docs.timescale.com/latest/getting-started/installation
* https://www.postgresql.org/download/

Для ненадійних або відкритих для мереж важливо захистити з'єднання PostgreSQL. Зверніться до документів PostgreSQL, щоб дізнатись, як налаштувати підключення за допомогою сертифікатів.

* https://www.postgresql.org/docs/12/ssl-tcp.html

Реплікація на резервний сервер рекомендується для високої доступності.

* https://www.postgresql.org/docs/12/different-replication-solutions.html
* https://www.postgresql.org/docs/12/warm-standby.html#STANDBY-SERVER-OPERATION

### 3. Графана

Графана, версія 7.1.x. Попередні версії можуть працювати, але не рекомендуються.

* https://grafana.com/grafana/download
* https://grafana.com/docs/grafana/latest/installation/

Якщо сертифікати налаштовані на з'єднання PostgreSQL із сервером, його також слід налаштувати у джерелі даних Grafana PostgreSQL для доступу до історичних даних.

### 4. Node.js

* Node.js версії 14.x. Попередні версії не перевіряються та не підтримуються.
* https://nodejs.org/en/

### 5. Голанг

* Golang, версія 1.14.x. Попередні версії не перевіряються та не підтримуються.
* https://golang.org/dl/

### 6. Ядро DotNet

* DotNet Core версії 3.1. Попередні версії не перевіряються та не підтримуються.
* https://dotnet.microsoft.com/download

### 7. Інші рекомендовані програмні засоби

* Inkscape SAGE або SCADAvis.io SVG Editor для створення синоптичного дисплея - https://sourceforge.net/projects/oshmiopensubstationhmi/ або https://www.microsoft.com/en-us/p/scadavisio-synoptic-editor/9p9905hmkz7x . Доступно лише для Windows.
* MongoDB Compass - https://www.mongodb.com/products/compass
* Git - https://git-scm.com/
* Код Visual Studio - https://code.visualstudio.com/
* Супервайзер (для Linux) - http://supervisord.org/installing.html
* NSSM (для Windows) - https://nssm.cc/

## Процеси JSON-SCADA - Створення та налаштування

Завантажте код з репзіторію

* https://github.com/oblikonline/json-scada

Або зробіть клон git

    клон git https://github.com/oblikonline/json-scada

Побудуйте код (використовуйте перевернуті косі риски, розширення .exe та копіюйте замість cp у Windows, виберіть також адекватну цільову платформу Dotnet)
    
    cd json-scada
    mkdir bin

    cd src/lib60870.netcore
    dotnet publish --runtime linux-x64 -p:PublishReadyToRun=true -c Release -o ../../bin/

    cd ../dnp3/Dnp3Client
    dotnet publish --runtime linux-x64 -p:PublishReadyToRun=true -c Release -o ../../bin/

    export GOBIN=~/json-scada/bin
    cd ../calculations
    go get ./... 
    go build 
    cp calculations ../../bin/

    cd ../i104m
    go get ./... 
    go build 
    cp i104m ../../bin/

    cd ../cs_data_processor
    npm update
    cd ../server_realtime
    npm update
    cd ../oshmi2json
    npm update

Налаштуйте файл conf/json-scada.json, щоб визначити ім'я вузла та вказати на сервер MongoDB. Процеси шукатимуть файл конфігурації в папці ../conf/.

* [Документація до файлу конфігурації] (../conf/README.md)

Процеси можна розподіляти на різних серверах, кожен сервер повинен мати різне ім'я вузла.

На одному сервері можуть працювати кілька систем JSON-SCADA, для цього кожна повинна мати окрему базу даних MongoDB та PostgreSQL та окрему структуру папок. Також потрібно налаштувати окремі HTTP-порти прослуховування.

Рекомендується запускати процеси JSON-SCADA як служби або демони. У Linux рекомендується інструмент _Supervisor_ для управління процесами. У Windows рекомендується конвертувати процеси в службах Windows за допомогою інструменту NSSM.

### Конфігурація супервізора (Linux)

Встановіть супервізор для вашої ОС

* http://supervisord.org/installing.html

Налаштуйте файл _/etc/supervisord.conf_ для управління процесами JSON-SCADA.

    ; Sample supervisor config file.
    ;
    ; For more information on the config file, please see:
    ; http://supervisord.org/configuration.html
    ;

    [inet_http_server]         ; inet (TCP) server disabled by default
    port=127.0.0.1:9000        ; ip_address:port specifier, *:port for all iface
    username=jsonscada         ; default is no username (open server)
    password=secret            ; default is no password (open server)

    [supervisord]
    logfile=/tmp/supervisord.log ; main log file; default $CWD/supervisord.log
    logfile_maxbytes=50MB        ; max main logfile bytes b4 rotation; default 50MB
    logfile_backups=10           ; # of main logfile backups; 0 means none, default 10
    loglevel=info                ; log level; default info; others: debug,warn,trace
    pidfile=/tmp/supervisord.pid ; supervisord pidfile; default supervisord.pid
    nodaemon=false               ; start in foreground if true; default false
    silent=false                 ; no logs to stdout if true; default false
    minfds=1024                  ; min. avail startup file descriptors; default 1024
    minprocs=200                 ; min. avail process descriptors;default 200

    ; The rpcinterface:supervisor section must remain in the config file for
    ; RPC (supervisorctl/web interface) to work.  Additional interfaces may be
    ; added by defining them in separate [rpcinterface:x] sections.

    [rpcinterface:supervisor]
    supervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface

    ; The supervisorctl section configures how supervisorctl will connect to
    ; supervisord.  configure it match the settings in either the unix_http_server
    ; or inet_http_server section.

    [supervisorctl]
    serverurl=unix:///tmp/supervisor.sock ; use a unix:// URL  for a unix socket
    ;serverurl=http://127.0.0.1:9001 ; use an http:// url to specify an inet socket
    ;username=user               ; should be same as in [*_http_server] if set
    ;password=123                ; should be same as in [*_http_server] if set
    ;prompt=mysupervisor         ; cmd line prompt (default "supervisor")
    ;history_file=~/.sc_history  ; use readline history if available

    [program:server_realtime]
    command=/usr/bin/node /home/jsuser/json-scada/src/server_realtime/index.js
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/src/server_realtime/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/server_realtime.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/server_realtime.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

    [program:cs_data_processor]
    command=/usr/bin/node /home/jsuser/json-scada/src/cs_data_processor/cs_data_processor.js
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/src/cs_data_processor/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/cs_data_processor.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/cs_data_processor.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

    [program:calculations]
    command=/home/jsuser/json-scada/bin/calculations
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/bin/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/calculations.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/calculations.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

    [program:iec104client]
    command=/home/jsuser/json-scada/bin/iec104client 1 1
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/bin/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/iec104client.log    ; stdout log path, NONE for none; 
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/iec104client.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

    [program:iec104server]
    command=/home/jsuser/json-scada/bin/iec104server 1 1
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/bin/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/iec104server.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/iec104server.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

    [program:pg_hist]
    command=/home/jsuser/json-scada/sql/process_pg_hist.sh
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/sql/   ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/pg_hist.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/pg_hist.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

    [program:pg_rtdata]
    command=/home/jsuser/json-scada/sql/process_pg_rtdata.sh
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/sql/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/pg_rtdata.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/pg_rtdata.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

Виконайте демон супервізора

     supervisord -c /etc/supervisord.conf

Використовуйте інструмент менеджера для запуску, зупинки та контролю системи

    supervisorctl
        start all
        status
        tail -f cs_data_processor
        help

### Конфігурація NSSM (Windows)

Встановіть інструмент NSSM. Його можна встановити в c:\json-scada\bin\.

За допомогою інструменту створіть необхідні сервіси.

    cd c:\json-scada\bin
    nssm install JSON_SCADA_calculations "C:\json-scada\bin\calculations.exe"
    nssm install JSON_SCADA_iec104server "C:\json-scada\bin\iec104server.exe"
    nssm install JSON_SCADA_iec104client "C:\json-scada\bin\iec104client.exe"
    nssm install JSON_SCADA_cs_data_processor <PATH_TO_NODEJSEXE>\node "C:\json-scada\src\cs_data_processor\cs_data_processor.js"
    nssm install JSON_SCADA_server_realtime <PATH_TO_NODEJSEXE>\node "C:\json-scada\src\server_realtime\index.js"

   ... і так далі ...

Для управління послугами використовуйте

    nssm start service_name
    nssm stop service_name
    nssm restart service_name
    nssm status service_name
    nssm remove service_name

   
