deploy - a shitty (but strangely useful) deployment tool written in go

## Synopsis
`deploy [--stdout] [--target <file>] [--script <file>]`
```
	--stdout
		pipe remote shell stdout to current shell stdout
	--target <file>
		ppath to the file with JSON-formatted targets (default "target.json")
	--script <file>
		path to the shell script file (default "script.sh")
```

## Description
`deploy` reads a JSON-formatted targets file and starts a remote SSH shell session for each valid target entry.
A specified shell script is then piped to the remote shell and executed.

## Concept
Deploy tries to help with installing/updating/../ your programs by introducing `targets` and `scripts`, which have a simple relation: a `script` is run on a `target`.
When running the tool you specify a targets file and a script file (or use default file names).
The tool connects to each target and executes the script, concurrently.

**What is a target?**
A target is any (SSH-able) machine you want to access: fridge, rpi, school computer, website server, etc.
Each targets file can be described as a JSON array with target objects, specifying the username, address:port and authentication method:
```JSON
[
	{
		"username": "bob",
		"host": "myserver:22",
		"auth": {
			"method": "password" or "pki",
			"artifact": "<secret>" or "/path/to/private_key.pem"
	 	}
	}
]
```

**What is a script?**
A script is basically a shell script file that is run on a target:
```shell
echo 'hello from the other side'
```

## Scenarios
Here are some scenarios and ideas for target and script files:
```
one program, many servers
-------------------------

				 [server 1]		specify one target (with all targets) and script file.
[program]  --->  [server 2]
				 [server N]		ex: deploy -target frontend -script website
								ex: deploy -target pi_cluster -script compute_pi
many programs, many servers
---------------------------

[program 1]  --->  [server 1]	specify one target file per server, and one script file per program.
[program 2]  --->  [server 2]
[program N]	 --->  [server N]	ex: deploy -target school -script compile_labs
								ex: deploy -target fridge -script got_ice

many programs, one server
-------------------------

[program 1]  --->  [server 1]	specify one target file, and one script file per program.
[program 2]  --->  [server 1]
[program N]	 --->  [server 1]	ex: deploy -target database -script backup_database
								ex: deploy -target rpi -script coffee_watch

one program, one server
-----------------------

[program]    --->  [server]		specify one target and script file.
								ex: deploy -target server -script boostrap
```

Closing thought: This seems more like a ssh-based, remote shell scripting tool than a deploy tool...

## LICENSE
MIT (see LICENSE file)
