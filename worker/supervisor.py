"""Resident worker supervisor: keep one worker.py alive forever.

Runs windowless (pythonw); restarts the worker on any exit. All output
goes to <label>.log in this directory (stdout and stderr).
"""
import os
import subprocess
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))


def main(argv):
    label = argv[1] if len(argv) > 1 else "worker"
    log = open(os.path.join(HERE, "%s.log" % label), "a", encoding="utf-8")
    log.write("[supervisor] watching %s\n" % label)
    log.flush()
    while True:
        log.write("[supervisor] start %s\n" % time.strftime("%H:%M:%S"))
        log.flush()
        try:
            proc = subprocess.run(
                [sys.executable, "worker.py"], cwd=HERE,
                stdout=log, stderr=subprocess.STDOUT)
            log.write("[supervisor] exit %s\n" % proc.returncode)
        except Exception as exc:  # noqa: BLE001 - supervisor never dies
            log.write("[supervisor] spawn failed: %s\n" % exc)
        log.flush()
        time.sleep(5)


if __name__ == "__main__":
    main(sys.argv)
