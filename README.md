# Go Bindings for Intel&reg; Quick Assist Technology Library

## Overview
QATgo provides Go bindings for Intel&reg; QAT user mode libraries

Intel&reg; QuickAssist Technology (Intel&reg; QAT) provides hardware acceleration for offloading security, authentication and compression services from the CPU, thus significantly increasing the performance and efficiency of standard platform solutions.

## Features
The following services are available in QATgo v1.0.0:

* Compression (qatgo/qatzip)
  * DEFLATE gzip and raw (QAT 1.x, 2.0)
  * lz4 (QAT 2.0)
  * zstd (QAT 2.0)
  * Compress and Verify (CnV)
  * Compress and Verify and Recover (CnVnR)
  * End-to-end (E2E) integrity check

## Supported Devices
* C62x (QAT C62x series chipset) QAT 1.x
* 4xxx (QAT gen 4 devices) QAT 2.0

## Software Requirements
* Go 1.18 or above: https://go.dev
* QATzip library 1.1.2 or above: https://github.com/intel/QATzip
  * Requirements for QATzip: https://github.com/intel/QATzip/blob/master/README.md#software-requirements
* Optional: Intel zstd QAT Plugin (required for zstd): https://github.com/intel/QAT-ZSTD-Plugin
* Optional: libzstd v1.5.5 (required for zstd plugin): https://github.com/facebook/zstd

## Changelog
* First release of QATgo

## Release Notes
* QAT v1.x only supports compression levels 1-8
* QAT v2.0 supports compression level 1-12
* QAT produces multisession files
  * See: https://www.gnu.org/software/gzip/manual/html_node/Advanced-usage.html 
  * Supported by GNU gzip, Yann Collet lz4 and zstd utilities/libraries and Go compress/gzip
  * pierrec/lz4 does not currently support multisession files
* Output buffer growth is currently unbounded
* QAT zstd plugin only supports compression, decompression is done in software (libzstd)
* QAT zstd compression level > 12 is software only (libzstd)
