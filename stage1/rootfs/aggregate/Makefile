S1=stage1
S1ACI=../../../bin/$(S1).aci

$(S1ACI): aggregate.sh Makefile scripts/* units/* install.d/* aci-manifest
	@./aggregate.sh && cp aci-manifest $(S1)/manifest && ${ACTOOL} --debug build --no-compression --overwrite $(S1) $(S1ACI)

.PHONY: clean
clean:
	rm -Rf $(S1) $(S1ACI)

test:
	echo TODO
