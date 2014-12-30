BIN=enter
SRC=enter.c

$(BIN): $(SRC) Makefile
	$(CC) $(CFLAGS) -o $@ $(SRC) -static -s

.PHONY: clean
clean:
	rm -f $(BIN)

test:
	echo TODO
