echo THIS WILL ERASE CURRENT CONTENTS OF POSTGRESQL DATABASE! 
echo PRESS CTRL+C to STOP or any othe key to proceed!
pause
c:\json-scada\platform-windows\postgresql-runtime\bin\initdb.exe -D c:\json-scada\platform-windows\postgresql-data --username=postgres --auth=trust -E UTF8 --locale=en_US.UTF-8
