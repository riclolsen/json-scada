# Project IDX Setup

Start a free instance for dev/test on Google's Project IDX platform with just a Google account.

1 - Access the Project IDX platform, click on the "Get Started" button

    https://idx.dev

2 - Create a new Workspace importing a GitHub repository.

    URL: https://github.com/riclolsen/json-scada
    Name: json-scada

3 - Wait for the workspace to be imported and built. This will take a while, do not click the recover button.

4 - When started some terminals will open for initialize and build the project.

5 - Wait until the tasks are finished and the workspace is ready. This will take some minutes.

6 - Click the Project IDX button on left sidebar and select "Backend Ports".   

7 - Click the "Open New Window" action for port 8080. This will give access to the web UI for the project. Login with admin/jsonscada credentials.

8 - On the terminal control JSON-SCADA processes with the "supervisorctl" command. 

    supervisorctl status
    supervisorctl start all
    supervisorctl stop all
    supervisorctl restart all
    supervisorctl start iec104client
    supervisorctl start iec104client
    supervisorctl tail -f iec104client
