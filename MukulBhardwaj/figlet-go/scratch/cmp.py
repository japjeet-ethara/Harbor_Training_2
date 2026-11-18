import os, subprocess, sys

args = sys.argv[1:]
env = dict(os.environ, PYTHONIOENCODING="utf-8")
orig = subprocess.run([sys.executable, "-m", "pyfiglet", *args],
                      cwd="../pyfiglet", capture_output=True, env=env).stdout
mine = subprocess.run(["tmp/figlet.exe", *args], capture_output=True).stdout
print("ORIGINAL:", repr(orig.decode("utf-8", "replace")))
print("MINE:    ", repr(mine.decode("utf-8", "replace")))
print("MATCH" if orig == mine else "DIFFERENT")