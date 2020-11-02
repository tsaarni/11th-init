#!/usr/bin/env -S python3 -u

import os
import time

pid = os.fork()
if pid != 0:
  print("Child pid={}".format(pid))
  time.sleep(999999)
else:
  time.sleep(1)
  # child forks grandchild and exits 
  pid2 = os.fork()
  if pid2 != 0:
    print("Grandchild pid={}".format(pid2))
    time.sleep(5)
    print("Child exits and grandchild becomes zombie")
  else:
    # grandchild exits and becomes zombie
    pass
