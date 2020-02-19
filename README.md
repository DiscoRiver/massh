![logo](./doc/logo.jpg)

Distributed shell commands via SSH

### Notes:
Please note that all of the behaviour below is subject to change. Development of this program is still very much in progress. 

The commands being run should be simple, or have basic output. The primary purpose is for identifying inconsistencies in 
command output, for environments that should have consistency.

Right now you can either user this repo as-is, which provides simple usage, or you can import the massh
package and use your own behaviour for building and running the Config.

Output is limited to printing a slice of Results. Additional output processing is required for anything
fancy. Result is a struct containing the host, the command, and the output (which includes the newline). 
This should be enough information for all your basic output needs. No status codes are returned. 

When specifying a script, it's contents will be added to stdin, and then the following command will be
executed to run it on the remote machine;

```cat > outfile.sh && chmod +x ./outfile.sh && ./outfile.sh && rm ./outfile.sh```

A more elegant solution is probably necessary, but this seems to be the most performative. The ability to 
control the temp file would probably be enough, but more research is necessary. 

### Usage:

```
Usage of ./massh:
  -a string
    	Arguments for script
  -c string
    	Set remote command to run.
  -insecure
    	Set insecure key mode.
  -p string
    	Public key file.
  -s string
    	Path to script file. Overrides -c switch.
  -t int
    	Timeout for ssh. (default 10)
  -u string
    	Specify user for ssh.
  -w int
    	Specify amount of concurrent workers. (default 5)
```

### Documentation

* [GoDoc](https://godoc.org/github.com/DiscoRiver/massh/massh)



