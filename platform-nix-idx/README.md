# Project IDX Setup

Start a free instance for dev/test on Google's Project IDX platform with just a Google account.

1. Access the Project IDX platform, click on the "Get Started" button. Connect your Google account if asked.

    [https://idx.dev](https://idx.dev)

2. Create a new Workspace importing a GitHub repository.

    * URL: https://github.com/riclolsen/json-scada
    * Name: json-scada

    Alternatively, you can fork the repo on Github and import it from there.

3. Wait for the workspace to be imported and built. This will take a while, do not click the recover button. When started, some terminals will open to initialize and build the project. Wait until all the tasks are finished and the workspace is ready. This will take some minutes.

4. Click the Project IDX button on left sidebar and select "Backend Ports". Click the "Open New Window" action for port 8080. This will give access to the web UI for the project. Login with admin/jsonscada credentials.

5. On the VSCode's terminal, control JSON-SCADA processes with the "supervisorctl" command.  

```bash
    supervisorctl status
    supervisorctl start all
    supervisorctl stop all
    supervisorctl restart all
    supervisorctl stop iec104client
    supervisorctl start iec104client
    supervisorctl tail -f iec104client
```

Open the Gemini chat with Ctrl+Shift+Space.

Notice that the provided free VM is a constrained environment with limited resources: 8GB RAM, 1-core CPUs, 10GB Disk.

More info for Project IDX [here](https://developers.google.com/idx).