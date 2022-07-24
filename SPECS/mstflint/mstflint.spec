%{!?ibmadlib: %define ibmadlib libibmad-devel}
%{!?buildtype: %define buildtype "native"}
%{!?noinband: %define noinband 0}
%{!?nodc: %define nodc 0}
%{!?enablecs: %define enablecs 0}
%{!?nopenssl: %define nopenssl 0}
%{!?enablexml2: %define enablexml2 0}
%{!?enablefwmgr: %define enablefwmgr 0}
%{!?enableadbgenerictools: %define enableadbgenerictools 0}
%{!?CONF_DIR: %define CONF_DIR /etc/mstflint}

%define mstflint_python_tools %{_libdir}/mstflint/python_tools

%define _unpackaged_files_terminate_build 0
%define debug_package %{nil}

Summary:        Mellanox firmware burning application
Name:           mstflint
Version:        4.16.0
Release:        1%{?dist}
License:        GPLv2/BSD
Vendor:         Microsoft Corporation
Distribution:   Mariner
Group:          System Environment/Base
URL:            https://github.com/Mellanox/mstflint
Source:         https://github.com/Mellanox/mstflint/releases/download/v4.16.0-1/mstflint-4.16.0-1.tar.gz#/%{name}-%{version}.tar.gz

ExclusiveArch: i386 i486 i586 i686 x86_64 ia64 ppc ppc64 ppc64le arm64 aarch64

%define openssl_devel_lib openssl-devel
%define expat_devel_lib expat-devel
%define boost_filesystem_lib boost-filesystem
%define boost_regex_lib boost-regex

%if "%{nopenssl}" == "0"
BuildRequires: %{openssl_devel_lib}
%endif

%if "%{enableadbgenerictools}" == "1"
BuildRequires: boost-devel
BuildRequires: %{expat_devel_lib}
BuildRequires: xz-devel
%endif

BuildRequires: zlib-devel %{ibmadlib}

%description
This package contains firmware update tool, vpd dump and register dump tools
for network adapters based on Mellanox Technologies chips.

%prep
%autosetup -p1

%build

MSTFLINT_VERSION_STR="%{name} %{version}-%{release}"

%if %{nodc}
    config_flags="$config_flags --disable-dc"
%endif

%if %{noinband}
    config_flags="$config_flags --disable-inband"
%endif

%if %{nopenssl}
    config_flags="$config_flags --disable-openssl"
%endif

%if %{enablecs}
    config_flags="$config_flags --enable-cs"
%endif

%if %{enablexml2}
    config_flags="$config_flags --enable-xml2"
%endif

%if %{enablefwmgr}
    config_flags="$config_flags --enable-fw-mgr"
%endif

%if %{enableadbgenerictools}
    config_flags="$config_flags --enable-adb-generic-tools"
%endif

%if %{buildtype} == "ppc"
    config_flags="$config_flags --host=ppc-linux"
%endif

%if %{buildtype} == "ppc64"
    config_flags="$config_flags --host=ppc64-linux --enable-static-libstdcpp=yes"
%endif

%if %{buildtype} == "ppc64le"
    config_flags="$config_flags --host=powerpc64le-linux-gnu --enable-dynamic-ld=yes"
%endif

%if %{buildtype} == "arm64"
    config_flags="$config_flags --host arm"
%endif

%configure ${config_flags} MSTFLINT_VERSION_STR="${MSTFLINT_VERSION_STR}"

make

%install
rm -rf %{buildroot}
make DESTDIR=%{buildroot} install
# remove unpackaged files from the buildroot
rm -f %{buildroot}%{_libdir}/*.la

# create softlinks to old mtcr header and lib locations
# link mtcr_ul to old location
mkdir -p %{buildroot}/%{_includedir}/mtcr_ul
ln -s %{_includedir}/mstflint/mtcr.h %{buildroot}/%{_includedir}/mtcr_ul/mtcr.h
ln -s %{_includedir}/mstflint/mtcr_com_defs.h %{buildroot}/%{_includedir}/mtcr_ul/mtcr_com_defs.h
# link mtcr_ul to old lib path
ln -s %{_libdir}/mstflint/libmtcr_ul.a %{buildroot}/%{_libdir}/libmtcr_ul.a 

%clean
rm -rf %{buildroot}

%files
%defattr(-,root,root)
%license COPYING
%{_bindir}/mstmread
%{_bindir}/mstmwrite
%{_bindir}/mstflint
%{_bindir}/mstregdump
%{_bindir}/mstmtserver
%{_bindir}/mstvpd
%{_bindir}/mstmcra
%{_bindir}/mstconfig
%{_bindir}/mstfwreset
%{_bindir}/mstcongestion
%{_bindir}/mstprivhost
%{_bindir}/mstfwtrace
%{_bindir}/mstresourcedump
%{_bindir}/mstresourceparse
%if %{enablefwmgr}
 %{_bindir}/mstfwmanager
 %{_bindir}/mstarchive
%{CONF_DIR}/ca-bundle.crt
%endif
%if %{enableadbgenerictools}
 %{_bindir}/mstreg
 %{_bindir}/mstlink
%endif

%{_includedir}/mstflint/cmdif/icmd_cif_common.h
%{_includedir}/mstflint/cmdif/icmd_cif_open.h
%{_includedir}/mstflint/common/compatibility.h
%{_includedir}/mstflint/mtcr.h
%{_includedir}/mstflint/mtcr_com_defs.h
%{_includedir}/mstflint/mtcr_mf.h
%{_includedir}/mstflint/tools_layouts/adb_to_c_utils.h
%{_includedir}/mstflint/tools_layouts/icmd_layouts.h
%{_includedir}/mtcr_ul/mtcr.h
%{_includedir}/mtcr_ul/mtcr_com_defs.h
%{_libdir}/mstflint/libmtcr_ul.a
%{_libdir}/libmtcr_ul.a
%{_libdir}/libcmdif.a
%{_libdir}/libdev_mgt.a
%{_libdir}/libreg_access.a
%{_libdir}/libtools_layouts.a

%{mstflint_python_tools}/tools_version.py
%{mstflint_python_tools}/mft_logger.py
%{mstflint_python_tools}/mlxpci/*.py
%{mstflint_python_tools}/mstfwtrace/*.py
%{mstflint_python_tools}/mstfwreset/mstfwreset.py
%{mstflint_python_tools}/mstfwreset/mlxfwresetlib/*.py
%{mstflint_python_tools}/mtcr.py
%{mstflint_python_tools}/cmtcr.so
%{mstflint_python_tools}/cmdif.py
%{mstflint_python_tools}/ccmdif.so
%{mstflint_python_tools}/regaccess.py
%{mstflint_python_tools}/rreg_access.so
%{mstflint_python_tools}/dev_mgt.py
%{mstflint_python_tools}/c_dev_mgt.so
%{mstflint_python_tools}/mstprivhost/mstprivhost.py

# Usually, python2 is the default python on a machine,
# so we want to ignore python2 erros caused by incompatiblity with python3 syntax
%define _python_bytecompile_errors_terminate_build 0

%{mstflint_python_tools}/mstresourcedump/*.py
%{mstflint_python_tools}/mstresourcedump/validation/*.py
%{mstflint_python_tools}/mstresourcedump/utils/*.py
%{mstflint_python_tools}/mstresourcedump/segments/*.py
%{mstflint_python_tools}/mstresourcedump/resource_data/*.py
%{mstflint_python_tools}/mstresourcedump/filters/*.py
%{mstflint_python_tools}/mstresourcedump/fetchers/*.py
%{mstflint_python_tools}/mstresourcedump/commands/*.py
%{mstflint_python_tools}/mstresourceparse/*.py
%{mstflint_python_tools}/mstresourceparse/parsers/*.py
%{mstflint_python_tools}/mstresourceparse/utils/*.py
%{mstflint_python_tools}/mstresourceparse/segments/*.py
%{mstflint_python_tools}/mstresourceparse/resource_data/*.py

%{_datadir}/mstflint
%{_mandir}/man1/*

%changelog
* Thu Jun 23 2022 Rachel Menge <rachelmenge@microsoft.com> - 4.16.0-1
- Initial CBL-Mariner import from NVIDIA (license: ASL 2.0)
- License verified

* Thu Dec 17 2020 Konstantin Maksatov <kostiantynm@nvidia.com>
- MFT 4.16.0 Updates.

* Wed Jun 24 2020 Konstantin Maksatov <kostiantynm@nvidia.com>
- Added BlueField-2 adapter cards support.

* Tue Mar 31 2020 Eran Jakoby <eranj@dev.mellanox.co.il>
- MFT 4.14.0 Updates.

* Tue Dec 31 2019 Eran Jakoby <eranj@dev.mellanox.co.il>
- MFT 4.13.3 Updates. Added new tools: mstresourcedump.

* Mon Sep 23 2019 Dan Goldberg <dang@dev.mellanox.co.il>
- Added conditional lib name of the dependencies to support both RH and SLES naming conventions.

* Wed May 22 2019 Eran Jakoby <eranj@dev.mellanox.co.il>
- MFT 4.12.0 Updates. Added new tools: mstreg, mstfwtrace and mstlink.

* Wed Nov 21 2018 Dan Goldberg <dang@dev.mellanox.co.il>
- MFT 4.11.0 Updates. Added new tools: mlxarchive and mstprivhost.

* Sun Jul 01 2018 Dan Goldberg <dang@dev.mellanox.co.il>
- MFT 4.10.0 Updates

* Mon Jul 17 2017 Adham Masarwah <adham@dev.mellanox.co.il>
- Adding mlxfwreset to mstflint
   
* Mon May 08 2017 Adrian Chiris <adrianc@dev.mellanox.co.il>
- MFT 4.7.0 Updates
   
* Tue Jan 10 2017 Adham Masarwah <adham@dev.mellanox.co.il>
- Removed hca_self_test.ofed installation from the package
    
* Mon Jan 9 2017 Adham Masarwah <adham@dev.mellanox.co.il>
- MFT 4.6.0 Updates
    
* Tue May 17 2016 Adrian Chiris <adrianc@dev.mellanox.co.il>
- MFT 4.4.0 Updates
   
* Wed Mar 23 2016 Adrian Chiris <adrianc@dev.mellanox.co.il>
- MFT 4.4.0 Updates

* Mon Jan 11 2016 Adrian Chiris <adrianc@dev.mellanox.co.il>
- MFT 4.3.0 Updates

* Wed Aug 26 2015 Adrian Chiris <adrianc@dev.mellanox.co.il>
- MFT 4.1.0 Updates
   
* Fri Jun 05 2015 Adrian Chiris <adrianc@dev.mellanox.co.il>
- MFT 4.0.1 Updates

* Thu Feb 05 2015 Adrian Chiris <adrianc@dev.mellanox.co.il>
- MFT 4.0.0 Updates

* Sun Dec 07 2014 Tomer Cohen <tomerc@mellanox.com>
- Added support for multiple architectures

* Sun Oct 12 2014 Oren Kladnitsky <orenk@dev.mellanox.co.il>
- MFT 3.7.1

* Thu Jul 31 2014 Oren Kladnitsky <orenk@dev.mellanox.co.il>
- MFT 3.7.0 Updates

* Mon Mar 31 2014 Oren Kladnitsky <orenk@dev.mellanox.co.il>
- MFT 3.6.0 Updates

* Tue Dec 24 2013 Oren Kladnitsky <orenk@dev.mellanox.co.il>
- MFT 3.5.0 Updates

* Wed Mar 20 2013 Oren Kladnitsky <orenk@dev.mellanox.co.il>
- MFT 3.0.0

* Thu Dec  4 2008 Oren Kladnitsky <orenk@dev.mellanox.co.il>
- Added hca_self_test.ofed installation
   
* Sun Dec 23 2007 Oren Kladnitsky <orenk@dev.mellanox.co.il>
- Added mtcr.h installation
   
* Fri Dec 07 2007 Ira Weiny <weiny2@llnl.gov> 1.0.0
- initial creation
