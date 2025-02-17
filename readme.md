# Monitor Monkey Agent

The deploy.sh script will create a user called monitormonkey on your system and
install the agent. It will run as a systemd service called monitor-monkey

If you wish to manually install this agent, download this repo  and compile it
using go. To run you will need to make sure MONKEY_API_KEY is set in your env

Alternatively you can download the deploy.sh script and modify the vars in the
script to configure the installation.

There is an uninstaller script provided. Change vars in this one to match your
custom install if needed.
