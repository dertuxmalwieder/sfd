[![Scc Count Badge](https://sloc.xyz/github/dertuxmalwieder/sfd?category=code)](https://github.com/dertuxmalwieder/sfd) [![Donate](https://img.shields.io/badge/Donate-PayPal-green.svg)](https://paypal.me/GebtmireuerGeld)

# sfd

An Single File (Web) Downloader.

## Features

* Downloads a complete website as self-contained HTML files, one file per requested page.

## Building

    fossil clone https://code.rosaelefanten.org/sfd

or

    git clone https://github.com/dertuxmalwieder/sfd

then (if applicable) go to the source directory, then

    go build

## Usage

Assuming there is a manual page with shiny pictures on `example.com/shiny-manual.htm` which you want to download into the current directory:

    sfd -url "https://example.com/shiny-manual.htm" -target .

`-target` defaults to your home directory.

