pipeline {
    agent any

    environment {
        DEPLOY_PATH = '/signerflow/crl'
        SERVICE_NAME = 'signerflow-crl'
        GO_VERSION = '1.21'
    }

    stages {
         stage('Clone Repository') {
            steps {
                echo '📥 Clonando repositorio...'
                git branch: "main",
                    url: "https://github.com/jmcuenca/signerflow-crl",
                    credentialsId: 'signerflow'
            }
        }

        stage('Build') {
            steps {
                script {
                    sh '''
                        export PATH=$PATH:/usr/local/go/bin
                        go mod download
                        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o signerflow-crl-service .
                    '''
                }
            }
        }

        stage('Prepare Deployment Package') {
            steps {
                script {
                    sh '''
                        mkdir -p deploy
                        cp signerflow-crl-service deploy/
                        cp crl_urls.json deploy/
                        cp ecosystem.config.js deploy/
                        cp .env deploy/
                    '''
                }
            }
        }

        stage('Deploy') {
            steps {
                script {
                    def timestamp = sh(script: "date +%Y%m%d_%H%M%S", returnStdout: true).trim()
                    def versionPath = "${DEPLOY_PATH}/versions/${timestamp}"

                    sh """
                        # Create directories on server
                        mkdir -p ${DEPLOY_PATH}/versions
                        mkdir -p ${versionPath}

                        # Copy files to versioned directory
                        cp -r deploy/* ${versionPath}/

                        # Remove old current symlink if exists
                        rm -f ${DEPLOY_PATH}/current

                        # Create new symlink to latest version
                        ln -s ${versionPath} ${DEPLOY_PATH}/current

                        # Set executable permissions
                        chmod +x ${DEPLOY_PATH}/current/signerflow-crl-service

                        # Restart service with PM2
                        cd ${DEPLOY_PATH}/current
                        
                        # Keep only last 5 versions
                        cd ${DEPLOY_PATH}/versions
                        ls -t | tail -n +6 | xargs -r rm -rf

                        echo "Deployment completed successfully!"
                        echo "Version: ${timestamp}"
                    """
                }
            }
        }

        
    }

    
}
