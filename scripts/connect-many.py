#!/usr/bin/python3
import sys
import resource
import socket
import ssl
import time

host, port = sys.argv[1].split(":")
addr = (host, int(port))
soft, hard = resource.getrlimit(resource.RLIMIT_NOFILE)
# reset soft == hard
resource.setrlimit(resource.RLIMIT_NOFILE, (hard, hard))

conns = []
t0 = time.time()
try:
    for i in range(soft+100):
        s=socket.socket()
        w = ssl.wrap_socket(s, ssl_version=ssl.PROTOCOL_TLSv1)
        w.settimeout(1)
        w.connect(addr)
        conns.append(w)
        w.send(b"x")
except Exception as e:
    print("%s|%d|%s" % (e, len(conns), time.time()-t0))
    sys.exit(0)

print("UNTROUBLED|%d" % len(conns))
sys.exit(1)
