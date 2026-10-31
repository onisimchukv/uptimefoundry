# Makefile for UptimeFoundry Agent SELinux Policy Module
MODULE = uptimefoundry_agent
SHARE = /usr/share/selinux/devel

all: $(MODULE).pp

$(MODULE).pp: $(MODULE).te $(MODULE).fc
	@if [ -f $(SHARE)/Makefile ]; then \
		make -f $(SHARE)/Makefile $(MODULE).pp; \
	else \
		checkmodule -M -m -o $(MODULE).mod $(MODULE).te; \
		semodule_package -o $(MODULE).pp -m $(MODULE).mod -f $(MODULE).fc; \
	fi

clean:
	rm -rf *.mod *.pp tmp/

.PHONY: all clean
