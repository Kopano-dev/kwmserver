#!/usr/bin/env groovy

pipeline {
	agent {
		docker {
			image 'golang:1.8'
			args '-u 0'
		 }
	}
	environment {
		GLIDE_VERSION = 'v0.13.0'
		GLIDE_HOME = '/tmp/.glide'
		GOBIN = '/usr/local/bin'
	}
	stages {
		stage('Bootstrap') {
			steps {
				echo 'Bootstrapping..'
				sh 'curl -sSL https://github.com/Masterminds/glide/releases/download/$GLIDE_VERSION/glide-$GLIDE_VERSION-linux-amd64.tar.gz | tar -vxz -C /usr/local/bin --strip=1'
				sh 'go get -v github.com/golang/lint/golint'
				sh 'go get -v github.com/tebeka/go2xunit'
				sh 'go get -v github.com/axw/gocov/...'
				sh 'go get -v github.com/AlekSi/gocov-xml'
				sh 'go get -v github.com/wadey/gocovmerge'
			}
		}
		stage('Lint') {
			steps {
				echo 'Linting..'
				sh 'golint \$(glide nv) | tee golint.txt || true'
				sh 'go vet \$(glide nv) | tee govet.txt || true'
			}
		}
		stage('Build') {
			steps {
				echo 'Building..'
				sh 'make'
				sh './bin/kwmserverd version'
			}
		}
		stage('Test') {
			when {
				not {
					branch 'master'
				}
			}
			steps {
				echo 'Testing..'
				sh 'make test-xml-short'
			}
		}
		stage('Test with coverage') {
			when {
				branch 'master'
			}
			steps {
				echo 'Testing with coverage..'
				sh 'make test-coverage COVERAGE_DIR=test/coverage'
				publishHTML([allowMissing: false, alwaysLinkToLastBuild: true, keepAll: true, reportDir: 'test/coverage', reportFiles: 'coverage.html', reportName: 'Go Coverage Report HTML', reportTitles: ''])
				step([$class: 'CoberturaPublisher', autoUpdateHealth: false, autoUpdateStability: false, coberturaReportFile: 'test/coverage/coverage.xml', failUnhealthy: false, failUnstable: false, maxNumberOfBuilds: 0, onlyStable: false, sourceEncoding: 'ASCII', zoomCoverageChart: false])
			}
		}
		stage('Dist') {
			steps {
				echo 'Dist..'
				sh 'make dist'
			}
		}
	}
	post {
		always {
			archive 'dist/*.tar.gz'
			junit allowEmptyResults: true, testResults: 'test/*.xml'
			warnings parserConfigurations: [[parserName: 'Go Lint', pattern: 'golint.txt'], [parserName: 'Go Vet', pattern: 'govet.txt']], unstableTotalAll: '0'
			cleanWs()
		}
	}
}
