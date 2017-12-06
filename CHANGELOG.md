# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.3] - open
### Added
- Control http interface to trigger reload from the outside
- ```Bosnd``` can now be used without a swarm configuration to benefit from the templating without having Docker Swarm
- -v to display the version of the ```Bosnd```

### Changed
- **Warning** Breaking change in the Swarm configuration section! You have to specifiy the certificate parts individually!

## [v0.2] - 2017.11.30
### Added
- Prometheus metric support including configuration
- Added support to change debug pprof port

### Changed
- Minor configuration changes for debug section
- Refresh vendor dependencies
- Various documentation updates

## [v0.1] - 2017-11-29
### Added
- Initial load of the repository

### Changed

### Removed

[Unreleased]: https://github.com/n0r1sk/bosnd/compare/v0.3...HEAD
[v0.3]: https://github.com/n0r1sk/bosnd/compare/v0.2...v0.3
[v0.2]: https://github.com/n0r1sk/bosnd/compare/v0.1...v0.2

