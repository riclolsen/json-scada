-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 2.0.2

###	Changes
- Added support of .NET 8.0
- Removed support of .NET 4.6.2 and .NET 4.7.2
- NuGet packages are now only distributed locally in the repository

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 2.0.1

###	Changes
- Clarified license of the NuGet package (SCLA 1.0)

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 2.0.0

###	Breaking Changes
- Splitted into 3 DLLs.
 - Technosoftware.DaAeHdaClient.dll
 - Technosoftware.DaAeHdaClient.Com.dll
 - Technosoftware.OpcRcw.dll 
- Connection to a server is now done differently. See examples!

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 1.4.1

###	Changes
- Added .NET 7.0

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 1.4.0

###	Changes
- Merged https://github.com/technosoftware-gmbh/opcua-solution-net-samples into this repository
- NuGet packages are now also GPL 3.0 or SCLA 1.0

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 1.3.0

### Breaking Changes
- removed support of .NET 5.0 because of end of life (see [here](https://dotnet.microsoft.com/en-us/platform/support/policy/dotnet-core))

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 1.2.6

###	Changes
- Update to enable/disable DCOM call cancellation

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 1.2.5

###	Fixed Issues
- Support of .NET 6.0 

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 1.2.4

###	Fixed Issues
- Added README to nuget package
- Changed lock for subscription to instance lock to increase performance. Might affect callback handling. 

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 1.2.3

###	Fixed Issues
- For OPC DA 2.0 only servers Subscription GetState() was called wrong. The OPC DA 3.0 version was used instead of the DA 2.0 version.

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 1.2.2

###	Changes
- Added ApplicationInstance class containing Property TimAsUtc (from LicenseHandler) and InitializeSecurity
- InitializeSecurity can be used to set the authentication level to Integrity as requested in [KB5004442—Manage changes for Windows DCOM Server Security Feature Bypass (CVE-2021-26414)](https://support.microsoft.com/en-us/topic/kb5004442-manage-changes-for-windows-dcom-server-security-feature-bypass-cve-2021-26414-f1400b52-c141-43d2-941e-37ed901c769c)

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 1.2.0

###	Changes
- NuGet packages are now available under a commercial license

### Fixed Issues
- Issue #14: Fix Connect method(). It ended correctly even if connection is not established
- Issue #15: Disconnect() method is not executed correctly because of wrong implemented Dispose() methods.

### Refactoring
- Refactored Technosoftware.DaAeHdaClient.Da
- Refactored Technosoftware.DaAeHdaClient.Ae
- Refactored Technosoftware.DaAeHdaClient.Hda
- Removed OpcServerType class because it is not used at all

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 1.1.1

###	Changes
- Changed copyright year
- Examples are now using the NuGet packages

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 1.1.0

###	Enhancements
- Support of .NET 5.0
- Added nuget packages

-------------------------------------------------------------------------------------------------------------
## OPC DA/AE/HDA Solution .NET - 1.0.0902

###	Enhancements
- Support of .NET Standard 2.1
- Also supported: .NET 4.8, .NET 4.7.2, .NET 4.6.2
- Support of OPC Classic Core Components 108.41
- Enhanced COM call tracing for OPC DA 
- For missing required OPC Interfaces a NotSupportedException is thrown and for optional ones just an entry in the log created.

###	Redistributables
- Redistributables are available [here](https://opcfoundation.org/developer-tools/samples-and-tools-classic/core-components/)


