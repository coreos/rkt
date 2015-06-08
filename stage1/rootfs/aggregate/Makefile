S1=stage1
S1ACI=../../../bin/$(S1).aci

$(S1ACI): aggregate.sh Makefile scripts/* units/* install.d/* ../${USR}/aci-manifest
	@./aggregate.sh && cp ../${USR}/aci-manifest $(S1)/manifest && ${ACTOOL} build --overwrite $(S1) $(S1ACI)

.PHONY: clean
clean:
	rm -Rf $(S1) $(S1ACI)

test:
	echo TODO
