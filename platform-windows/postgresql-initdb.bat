echo THIS WILL ERASE CURRENT CONTENTS OF POSTGRESQL DATABASE! 
echo PRESS CTRL+C to STOP or any othe key to proceed!
pause
c:\json-scada\postgresql-runtime\bin\initdb.exe -D c:\json-scada\postgresql-data --username=json_scada --auth=trust -E UTF8 --locale=en_US.UTF-8
