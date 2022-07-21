# Theia Command-line Tool

`theia` is the command-line tool which provides access to Theia network flow
visibility capabilities.

## Table of Contents

<!-- toc -->
- [Installation](#installation)
- [Usage](#usage)
<!-- /toc -->

## Installation

`theia` binaries are published for different OS/CPU Architecture combionations.
For Linux, we also publish binaries for Arm-based systems. Refer to the
[releases page](https://github.com/antrea-io/theia/releases) and
download the appropriate one for your machine. For example:

On Mac & Linux:

```bash
curl -Lo ./theia "https://github.com/antrea-io/theia/releases/download/<TAG>/theia-$(uname)-x86_64"
chmod +x ./theia
mv ./theia /some-dir-in-your-PATH/theia
theia help
```

On Windows, using PowerShell:

```powershell
Invoke-WebRequest -Uri https://github.com/antrea-io/theia/releases/download/<TAG>/theia-windows-x86_64.exe -Outfile theia.exe
Move-Item .\theia.exe c:\some-dir-in-your-PATH\theia.exe
theia help
```

## Usage

To see the list of available commands and options, run `theia help`. Currently,
we have 5 commands for the NetworkPolicy Recommendation feature:

- `theia policy-recommendation run`
- `theia policy-recommendation status`
- `theia policy-recommendation retrieve`
- `theia policy-recommendation list`
- `theia policy-recommendation delete`

For details, please refer to [NetworkPolicy recommendation doc](
networkpolicy-recommendation.md)
