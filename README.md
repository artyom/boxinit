`boxinit` is a small program with init-like capabilities and is intended to be run as PID-1 process inside docker containers.

Features:

- relay signals to child processes: INT, HUP, TERM, QUIT, USR1, USR2
- run a couple of child processes and exit if any of them returns, terminating all other children
- adopt orphaned processes and reap them when they finish (solve zombie processes issue)

boxinit takes arbitrary number of arguments each of them treated as a name of the command to run (without parameters). Use wrapper scripts and environment variables if you need to configure complex start commands.

Example:

Consider you want to run two services inside your container and finish container if any of those two services terminates.

Create start script for each service holding configuration parameters, etc:

	cat service1.sh
	#!/bin/sh
	exec /usr/bin/my-service1 --config=/etc/service1.conf

	cat service2.sh
	#!/bin/sh
	exec /usr/bin/my-service2 --foo=bar

Configure your `Dockerfile` as following:

	COPY service*.sh boxinit /init/
	CMD [ "/init/boxinit", "/init/service1.sh", "/init/service2.sh" ]

This way `boxinit` would be your PID-1 process inside container, would relay essential signals to both services and would exit if either of them finish, allowing you to restart/heal contaier.
