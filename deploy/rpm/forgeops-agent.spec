Name:           forgeops-agent
Version:        1.3.0
Release:        1%{?dist}
Summary:        Zero-credential, eBPF-powered host telemetry and discovery agent for ForgeOps

License:        Apache-2.0
URL:            https://github.com/forgeops/forgeops
Source0:        forgeops-agent-%{version}.tar.gz

BuildRequires:  systemd-rpm-macros
BuildRequires:  golang >= 1.22
Requires(pre):  shadow-utils
Requires:       systemd

%description
ForgeOps Agent is a multi-process host agent providing zero-code eBPF observability,
cgroup-aware network flow tracking, procfs host discovery, and local security policy
ceiling enforcement without storing credentials on target hosts.

%package ebpf
Summary:        eBPF worker and probe runtime for ForgeOps Agent
Requires:       forgeops-agent = %{version}-%{release}

%description ebpf
The eBPF worker component for ForgeOps Agent. Runs with isolated AF_UNIX and AF_NETLINK
sockets and drops all network capabilities.

%prep
%setup -q

%build
go build -v -ldflags="-s -w -X main.version=%{version}" -o bin/forgeops-agent ./cmd/forgeops-agent
go build -v -ldflags="-s -w -X main.version=%{version}" -o bin/forgeops-agentctl ./cmd/forgeops-agentctl

%install
install -D -p -m 0755 bin/forgeops-agent %{buildroot}%{_bindir}/forgeops-agent
install -D -p -m 0755 bin/forgeops-agentctl %{buildroot}%{_bindir}/forgeops-agentctl

install -D -p -m 0644 deploy/systemd/forgeops-agent.service %{buildroot}%{_unitdir}/forgeops-agent.service
install -D -p -m 0644 deploy/systemd/forgeops-agent-ebpf.service %{buildroot}%{_unitdir}/forgeops-agent-ebpf.service
install -D -p -m 0644 deploy/systemd/forgeops-agent-discovery.service %{buildroot}%{_unitdir}/forgeops-agent-discovery.service
install -D -p -m 0644 deploy/systemd/forgeops-agent.conf %{buildroot}%{_tmpfilesdir}/forgeops-agent.conf

install -d -m 0750 %{buildroot}%{_sysconfdir}/forgeops-agent
install -D -p -m 0640 configs/policy.yaml %{buildroot}%{_sysconfdir}/forgeops-agent/policy.yaml

install -d -m 0700 %{buildroot}%{_sharedstatedir}/forgeops-agent
install -d -m 0750 %{buildroot}%{_rundir}/forgeops-agent

%pre
getent group forgeops-agent >/dev/null || groupadd -r forgeops-agent
getent passwd forgeops-agent >/dev/null || \
    useradd -r -g forgeops-agent -d %{_sharedstatedir}/forgeops-agent -s /sbin/nologin \
    -c "ForgeOps Agent Supervisor" forgeops-agent
getent passwd forgeops-agent-disc >/dev/null || \
    useradd -r -g forgeops-agent -s /sbin/nologin \
    -c "ForgeOps Agent Discovery Helper" forgeops-agent-disc
getent passwd forgeops-agent-ebpf >/dev/null || \
    useradd -r -g forgeops-agent -s /sbin/nologin \
    -c "ForgeOps Agent eBPF Worker" forgeops-agent-ebpf
exit 0

%post
%systemd_post forgeops-agent.service forgeops-agent-ebpf.service forgeops-agent-discovery.service
systemd-tmpfiles --create forgeops-agent.conf || :

%preun
%systemd_preun forgeops-agent.service forgeops-agent-ebpf.service forgeops-agent-discovery.service

%postun
%systemd_postun_with_restart forgeops-agent.service

%files
%{_bindir}/forgeops-agent
%{_bindir}/forgeops-agentctl
%{_unitdir}/forgeops-agent.service
%{_unitdir}/forgeops-agent-discovery.service
%{_tmpfilesdir}/forgeops-agent.conf
%config(noreplace) %attr(0640, root, forgeops-agent) %{_sysconfdir}/forgeops-agent/policy.yaml
%dir %attr(0700, forgeops-agent, forgeops-agent) %{_sharedstatedir}/forgeops-agent
%ghost %dir %attr(0750, forgeops-agent, forgeops-agent) %{_rundir}/forgeops-agent

%files ebpf
%{_unitdir}/forgeops-agent-ebpf.service

%changelog
* Sat Oct 24 2026 ForgeOps Team <team@forgeops.io> - 1.3.0-1
- Initial release of ForgeOps Agent v1.3 with zero credentials and strict supervisor boundary
