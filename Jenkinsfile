#!/usr/bin/env groovy

pipeline {
	agent {
		dockerfile {
			filename 'Dockerfile.build'
		}
	}
	stages {
		stage('Bootstrap') {
			steps {
				echo 'Bootstrapping..'
				sh 'go version'
			}
		}
		stage('Lint') {
			steps {
				echo 'Linting..'
				sh 'make lint-checkstyle'
			}
		}
		stage('Test') {
			steps {
				echo 'Testing..'
				sh 'make test-xml-short'
			}
		}
		stage('Vendor') {
			steps {
				echo 'Fetching vendor dependencies..'
				sh 'make vendor'
			}
		}
		stage('Community') {
			stages {
				stage('Build community') {
					steps {
						echo 'Building..'
						sh 'make DATE=reproducible BUILD_TAGS=release'
						sh './bin/kwmserverd version && sha256sum ./bin/kwmserverd'
					}
				}
				stage('Dist community') {
					steps {
						echo 'Dist..'
						sh 'test -z "$(git diff --shortstat 2>/dev/null |tail -n1)" && echo "Clean check passed."'
						sh 'make check'
						sh 'make dist PACKAGE_NAME_SUFFIX=-community'
					}
				}
			}
		}
		stage('Supported') {
			stages {
				stage('Build supported') {
					steps {
						echo 'Building..'
						sh 'make DATE=reproducible BUILD_TAGS=release,supportedBuild'
						sh './bin/kwmserverd version && sha256sum ./bin/kwmserverd'
					}
				}
				stage('Dist supported') {
					steps {
						echo 'Dist..'
						sh 'test -z "$(git diff --shortstat 2>/dev/null |tail -n1)" && echo "Clean check passed."'
						sh 'make check'
						sh 'make dist'
					}
				}
			}
		}
		stage('Test with coverage') {
			steps {
				echo 'Testing with coverage..'
				sh 'make test-coverage COVERAGE_DIR=test/coverage.jenkins || true'
				publishHTML([allowMissing: true, alwaysLinkToLastBuild: true, keepAll: true, reportDir: 'test/coverage.jenkins', reportFiles: 'coverage.html', reportName: 'Go Coverage Report HTML', reportTitles: ''])
				step([$class: 'CoberturaPublisher', autoUpdateHealth: false, autoUpdateStability: false, coberturaReportFile: 'test/coverage.jenkins/coverage.xml', failUnhealthy: false, failUnstable: false, maxNumberOfBuilds: 0, onlyStable: false, sourceEncoding: 'ASCII', zoomCoverageChart: false])
			}
		}
	}
	post {
		always {
			junit allowEmptyResults: false, testResults: 'test/tests.xml'

			recordIssues enabledForFailure: true, qualityGates: [[threshold: 400, type: 'TOTAL', unstable: true]], tools: [checkStyle(pattern: 'test/tests.lint.xml')]

			archiveArtifacts 'dist/*.tar.gz'
			cleanWs()
		}
	}
}
