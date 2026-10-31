# Fedora / COPR spec: builds milehigh from the tagged source.
#   copr-cli build <your-project> packaging/fedora/milehigh.spec   (or use COPR's SCM source)
# Go modules are downloaded during the build, so enable networking for the COPR project.
%global debug_package %{nil}

Name:           milehigh
Version:        0.1.0
Release:        1%{?dist}
Summary:        Flight log for the terminal, inspired by Flighty
License:        GPL-2.0-or-later
URL:            https://github.com/lemonade1921/milehigh
Source0:        %{url}/archive/refs/tags/v%{version}/%{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.24

%description
milehigh keeps your flight history in the terminal: upcoming and past flights,
great-circle route maps, stats and a passport of visited countries. It reads
and writes Skyline backups, so the same log moves between your Android phone,
your Mac and your Linux machines.

%prep
%autosetup -n %{name}-%{version}

%build
export GOFLAGS="-buildmode=pie -trimpath -mod=readonly"
go build -ldflags "-s -w -X main.version=%{version}" -o %{name} ./cmd/milehigh

%check
go test ./...

%install
install -Dpm 0755 %{name} %{buildroot}%{_bindir}/%{name}

%files
%license LICENSE NOTICE
%doc README.md
%{_bindir}/%{name}

%changelog
* Sat Oct 31 2026 lemonade1921 - 0.1.0-1
- First release
