Smushing: The hierarchy rule was easiest to misdescribe; checking FigletBuilder showed the later class wins, and the final oracle matched all 2,000 hidden cases.
Docker: My first validation used `bash -lc`, which hid `/usr/local/go/bin` and failed at `gofmt`; switching to `bash -c` fixed it, and Bash is installed for Harbor's Alpine runner.
