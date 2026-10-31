Name:           uptimefoundry-agent
Version:        1.3.0
Release:        1%{?dist}
Summary:        Zero-credential, eBPF-powered host telemetry and discovery agent for UptimeFoundry

License:        MIT
URL:            https://github.com/onisimchukv/uptimefoundry
Source0:        uptimefoundry-agent-%{version}.tar.gz

BuildRequires:  systemd-rpm-macros
BuildRequires:  golang >= 1.22
Requires(pre):  shadow-utils
Requires:       systemd

%description
UptimeFoundry Agent is a multi-process host agent providing zero-code eBPF observability,
cgroup-aware network flow tracking, procfs host discovery, and local security policy
ceiling enforcement without storing credentials on target hosts.

%package ebpf
Summary:        eBPF worker and probe runtime for UptimeFoundry Agent
Requires:       uptimefoundry-agent = %{version}-%{release}

%description ebpf
The eBPF worker component for UptimeFoundry Agent. Runs with isolated AF_UNIX and AF_NETLINK
sockets and drops all network capabilities.

%prep
%setup -q

%build
go build -v -ldflags="-s -w -X main.version=%{version}" -o bin/uptimefoundry-agent ./cmd/uptimefoundry-agent
go build -v -ldflags="-s -w -X main.version=%{version}" -o bin/uptimefoundry-agentctl ./cmd/uptimefoundry-agentctl

%install
install -D -p -m 0755 bin/uptimefoundry-agent %{buildroot}%{_bindir}/uptimefoundry-agent
install -D -p -m 0755 bin/uptimefoundry-agentctl %{buildroot}%{_bindir}/uptimefoundry-agentctl

install -D -p -m 0644 deploy/systemd/uptimefoundry-agent.service %{buildroot}%{_unitdir}/uptimefoundry-agent.service
install -D -p -m 0644 deploy/systemd/uptimefoundry-agent-ebpf.service %{buildroot}%{_unitdir}/uptimefoundry-agent-ebpf.service
install -D -p -m 0644 deploy/systemd/uptimefoundry-agent-discovery.service %{buildroot}%{_unitdir}/uptimefoundry-agent-discovery.service
install -D -p -m 0644 deploy/systemd/uptimefoundry-agent.conf %{buildroot}%{_tmpfilesdir}/uptimefoundry-agent.conf

install -d -m 0750 %{buildroot}%{_sysconfdir}/uptimefoundry-agent
install -D -p -m 0640 configs/policy.yaml %{buildroot}%{_sysconfdir}/uptimefoundry-agent/policy.yaml

install -d -m 0700 %{buildroot}%{_sharedstatedir}/uptimefoundry-agent
install -d -m 0750 %{buildroot}%{_rundir}/uptimefoundry-agent

%pre
getent group uptimefoundry-agent >/dev/null || groupadd -r uptimefoundry-agent
getent passwd uptimefoundry-agent >/dev/null || \
    useradd -r -g uptimefoundry-agent -d %{_sharedstatedir}/uptimefoundry-agent -s /sbin/nologin \
    -c "UptimeFoundry Agent Supervisor" uptimefoundry-agent
getent passwd uptimefoundry-agent-disc >/dev/null || \
    useradd -r -g uptimefoundry-agent -s /sbin/nologin \
    -c "UptimeFoundry Agent Discovery Helper" uptimefoundry-agent-disc
getent passwd uptimefoundry-agent-ebpf >/dev/null || \
    useradd -r -g uptimefoundry-agent -s /sbin/nologin \
    -c "UptimeFoundry Agent eBPF Worker" uptimefoundry-agent-ebpf
exit 0

%post
%systemd_post uptimefoundry-agent.service uptimefoundry-agent-ebpf.service uptimefoundry-agent-discovery.service
systemd-tmpfiles --create uptimefoundry-agent.conf || :

%preun
%systemd_preun uptimefoundry-agent.service uptimefoundry-agent-ebpf.service uptimefoundry-agent-discovery.service

%postun
%systemd_postun_with_restart uptimefoundry-agent.service

%files
%{_bindir}/uptimefoundry-agent
%{_bindir}/uptimefoundry-agentctl
%{_unitdir}/uptimefoundry-agent.service
%{_unitdir}/uptimefoundry-agent-discovery.service
%{_tmpfilesdir}/uptimefoundry-agent.conf
%config(noreplace) %attr(0640, root, uptimefoundry-agent) %{_sysconfdir}/uptimefoundry-agent/policy.yaml
%dir %attr(0700, uptimefoundry-agent, uptimefoundry-agent) %{_sharedstatedir}/uptimefoundry-agent
%ghost %dir %attr(0750, uptimefoundry-agent, uptimefoundry-agent) %{_rundir}/uptimefoundry-agent

%files ebpf
%{_unitdir}/uptimefoundry-agent-ebpf.service

%changelog
* Sat Oct 24 2026 UptimeFoundry Team <team@example.com> - 1.3.0-1
- Initial release of UptimeFoundry Agent v1.3 with zero credentials and strict supervisor boundary
