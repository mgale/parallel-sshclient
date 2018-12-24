# parallel-sshclient

parallel-sshclient is designed to execute commands in parallel on remote hosts.

It provides the time taken to execute the command, the exit status and saves the output to /tmp/ for
later analysis.

Reasons behind creating parallel-sshlcient
* I am learning golang and needed something to build.
* I execute alot of ad-hoc commands against servers and usually end up running a bash for loop on
the command line. This can be slow as the server count increases. I did look into other solutions
like Ansible and parallel-ssh but decided to experiment with golang instead.

In the end I replaced this:
```
for s in `cat /tmp/production-host.list`; do echo ${s} ; user@${s} 'service <service-name> status'; done
```
with
```
./parallel-sshclient -hosts-file /tmp/production-host.list --remote-cmd "service <service-name> status" -curmax 50
```
---


## How to use parallel-sshclient

```
./parallel-sshclient -hosts-file /tmp/testfile --remote-cmd "ls -l" -curmax 50
```

## Sample Output
```
pssh:2018/12/23 15:17:16 Starting ....
pssh:2018/12/23 15:17:16 SSH Client example: ssh -i /Users/michaelgale/.ssh/id_rsa -p 22 michaelgale@<hostname>
pssh:2018/12/23 15:17:16 Loading hostnames file: /tmp/testfile
pssh:2018/12/23 15:17:16 Loaded 2 hosts
pssh:2018/12/23 15:17:16 Concurrency count: 5
pssh:2018/12/23 15:17:16 Remote Command: ls -l
pssh:2018/12/23 15:17:16 192.168.1.51 : ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain
pssh:2018/12/23 15:17:16 0.16s elapsed Host: 10.211.55.17 : CommandSuccess: true : output-saved: true
pssh:2018/12/23 15:17:16 Completed all hosts
pssh:2018/12/23 15:17:16 Total Hosts: 2    Commands Succesful: 1    Commands Failed:  1    Hosts Failed: 0
pssh:2018/12/23 15:17:16 0.16s Overall elapsed time
```