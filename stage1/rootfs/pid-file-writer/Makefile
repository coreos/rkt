BIN=pid-file-writer
SRC=pid-file-writer.c

$(BIN): $(SRC) Makefile install
	$(CC) $(CFLAGS) -o $@ $(SRC) -static -s
	@cp install ../aggregate/install.d/10pid-file-writer

.PHONY: clean
clean:
	rm -f $(BIN)

test:
	echo TODO
