deploy - insecure parallel ssh script execution

## Synopsis
```bash
deploy [--stdout] [--target file] [--script file]

Usage of deploy.exe:
  -script file
        path to the shell script file (default "script.sh")
  -stdout
        pipe remote shell stdout to current shell stdout
  -target file
        path to the file with JSON-formatted targets (default "target.json")
```

## How it works
 1 - SSH into [#target]($TARGET)
 2 - Execute [#script]($SCRIPT)
 3 - Done!

## target
```json
[
    {
        "username": "not-root",
        "host": "fridge.local:22",
        "auth": {
            "method": "pki",
            "artifact": "~/.ssh/id_rsa"
        }
    }
]
```
 - `username`: login user (w/interactive shell)
 - `host`: `(hostname|ip-addres):port`
 - `auth`: authentication scheme
 -  - `method`: `pki` or `password`
 -  - `artifact`: `path/to/id_rsa` or `user password`

note: it is not possible to verify the host key at the moment

## script
The script file contains the shell commands you want to invoke on each target.

