
echo THIS WILL ERASE CURRENT CONTENTS OF POSTGRESQL DATABASE!
echo PRESS CTRL+C to STOP or any othe key to proceed!
pause
..\postgresql-runtime\bin\initdb.exe -D postgresql-data --username=json_scada --auth=trust -E UTF8 --locale=en_US.UTF-8
