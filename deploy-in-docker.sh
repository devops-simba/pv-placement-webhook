#!/usr/bin/env sh

black="\033[0;30m"
darkGray="\033[1;30m"
red="\033[0;31m"
lightRed="\033[1;31m"
green="\033[0;32m"
lightGreen="\033[1;32m"
orange="\033[0;33m"
yellow="\033[1;33m"
blue="\033[0;34m"
lightBlue="\033[1;34m"
purple="\033[0;35m"
lightPurple="\033[1;35m"
cyan="\033[0;36m"
lightCyan="\033[1;36m"
lightGray="\033[0;37m"
white="\033[1;37m"
nc="\033[0m"

descColor=$darkGray
promptColor=$green
defValColor=$white
requiredColor=$orange
required="${orange}REQUIRED${nc}"

ApplicationDir=`realpath "$(dirname $0)"`
DefaultApplicationName=`echo $(basename "$ApplicationDir") | sed -e "s/_/-/g"`
DefaultNamespace="devops-webhooks"

args="--command deploy --folder deployment-scripts"

set_var() {
    variableName="$1"
    value="$2"
    eval "$variableName='$value'"
}
get_var() {
    eval "RESULT=\"\${$1}\""
}

# this function show a prompt to user and read its value from the user
# usage: read_option desc title default prev [result=title]
read_option() {
    desc="$1"
    title="$2"
    defaultValue="$3"
    prevValue="$4"
    maxLen="$5"
    resultVar="$6"
    resultVar="${resultVar:-${title}}"

    defval="${prevValue:-${defaultValue}}"
    if [ \( "$AUTO_RESPONSE" = "TRUE" \) -a \( -n "$defval" \) ]; then
        set_var "$resultVar" "$defval"
        return 0
    fi

    LengthIsOK=0
    if [ -n "$defval" ]; then
        echo "${descColor}${desc}${nc}"
        while [ $LengthIsOK -eq 0 ]; do
            echo -n "${promptColor}${title}${nc}(${defValColor}${defval}${nc}): "
            read RESULT
            RESULT="${RESULT:-${prevValue:-$defaultValue}}"
            if [ -n "$maxLen" ]; then
                if [ ${#RESULT} -gt $maxLen ]; then
                    echo "${red}Maximum allowed length for this variable is ${maxLen}${nc}"
                    continue
                fi
            fi
            if [ "$RESULT" = "$defaultValue" ]; then
                RESULT=""
            fi
            LengthIsOK=1
        done
    else
        # this option have no previous value nor any default value, so it is required
        echo "${descColor}${desc}${nc}."
        echo "${descColor}this parameter is ${requiredColor}REQUIRED${nc}"
        while [ -z "$RESULT" ]; do
            echo -n "${requiredColor}${title}${nc}: "
            read RESULT
            if [ \( -n $maxLen \) -a \(  ${#RESULT} -gt $maxLen \) ]; then
                echo "${red}Maximum allowed length for this variable is ${maxLen}${nc}"
                RESULT=""
            fi
        done
    fi
    set_var "$resultVar" "$RESULT"
}

echo "I will ask you a couple of question to configure the deployment."
echo "Almost all configurations have default value and you may accept it with just pressing ENTER"
echo ""

if [ -f "./.responses" ]; then
    . ./.responses
fi

read_option "Name of the application" "ApplicationName" "$DefaultApplicationName" "$ApplicationName" 11
if [ -n "$ApplicationName" ]; then
    args="$args --app '$ApplicationName'"
fi

read_option "Indicate whether server should run in insecure mode(just for debug)" "Insecure" "false" "$Insecure"
case $Insecure in
    yes|1|true|ok)
        Insecure="true" # make its value fixed
        args="${args} --insecure"
        echo "${orange}Warning: Insecure mode enabled${nc}"
        ;;
    *)
        Insecure=""
        read_option "Certificate file that should used by the webhook for secure connection" "Cetificate" "AUTO" "$Certificate"
        if [ -n "$Certificate" ]; then
            args="$args --cert '$Certificate'"

            read_option "Private key file that should used together with certificate" "PrivateKey" "" "$PrivateKey"
            args="$args --key '$PrivateKey'"

            read_option "If this certificate is signed by a custom CA, then you must provide the file that contains certificate of that CA here" "CAFile", "", "$CAFile"
            if [ -n "$CAFile" ]; then
                args="$args --ca '$CAFile'"
            fi
        fi

        read_option "Name of the secret that contains TLS secrets of the service" "SecretName" "$ApplicationName" "$SecretName"
        if [ -n "$SecretName" ]; then
            args="$args --secret-name '$SecretName'"
        fi
        ;;
esac

read_option "Name of the image that will be created from this application" "ImageName" "$ApplicationName" "$ImageName"
if [ -n "$ImageName" ]; then
    args="$args --image '$ImageName'"
fi

read_option "Tag of the created image" "ImageTag" "latest" "$ImageTag"
if [ -n "$ImageTag" ]; then
    args="$args --tag '$ImageTag'"
fi

read_option "Namespace that webhook should deployed to it" "WebhookNamespace" "$DefaultNamespace" "$WebhookNamespace" 15
if [ -n "$WebhookNamespace" ]; then
    args="$args --namespace '$WebhookNamespace'"
fi

SelectedNs="${WebhookNamespace:-$DefaultNamespace}"
DefaultPushRegistry="registry.apps.internal.ic.cloud.snapp.ir/$SelectedNs"
read_option "Registry that image should pushed to it. You must already be logged into this repository" "PushImageRegistry" "$DefaultPushRegistry" "$PushImageRegistry"
args="$args --push-registry '${PushImageRegistry:-$DefaultPushRegistry}'"

DefaultPullRegistry="image-registry.openshift-image-registry.svc:5000/$SelectedNs"
read_option "Registry that image should pulled from it." "PullImageRegistry" "$DefaultPullRegistry" "$PullImageRegistry"
args="$args --pull-registry '${PullImageRegistry:-$DefaultPullRegistry}'"

read_option "If you want to run the image as a non-root user, you must specify its user ID here" "RunAsUser" "1234" "$RunAsUser"
if [ -n "$RunAsUser" ]; then
    args="$args --runas '$RunAsUser'"
fi

read_option "Name of the service of this webhook" "ServiceName" "$ApplicationName" "$ServiceName" 15
if [ -n "$ServiceName" ]; then
    args="$args --service-name '$ServiceName'"
fi

read_option "User that should used for the service" "ServiceUser" "default user" "$ServiceUser"
if [ -n "$ServiceUser" ]; then
    args="$args --service-user '$ServiceUser'"
fi

read_option "Name of the secret that contains TLS certificate and key for this webhook" "SecretName" "$ApplicationName" "$SecretName" 15
if [ -n "$SecretName" ]; then
    args="$args --secret-name '$SecretName'"
fi

read_option "If building go application require a proxy to grab packages, write it here" "BuildProxy" "no proxy" "$BuildProxy"
if [ -n "$BuildProxy" ]; then
    args="$args --proxy '${BuildProxy}'"
fi

read_option "Level of logs of the running server" "LogLevel" "0" "$LogLevel"
if [ -n "$LogLevel" ]; then
    args="$args --level $LogLevel"
fi

read_option "kubectl command" "Kubectl" "kubectl" "$Kubectl"
if [ -n "$Kubectl" ]; then
    args="$args --kubectl '$Kubectl'"
fi

if [ -f "./.responses" ]; then
    rm -rf ./.responses
fi

# create a response cache
echo "ApplicationName='$ApplicationName'" >> ./.responses
echo "Insecure='$Insecure'" >> ./.responses
echo "Certificate='$Certificate'" >> ./.responses
echo "PrivateKey='$PrivateKey'" >> ./.responses
echo "CAFile='$CAFile'" >> ./.responses
echo "SecretName='$SecretName'" >> ./.responses
echo "ImageName='$ImageName'" >> ./.responses
echo "ImageTag='$ImageTag'" >> ./.responses
echo "PushImageRegistry='$PushImageRegistry'" >> ./.responses
echo "PullImageRegistry='$PullImageRegistry'" >> ./.responses
echo "RunAsUser='$RunAsUser'" >> ./.responses
echo "WebhookNamespace='$WebhookNamespace'" >> ./.responses
echo "ServiceName='$ServiceName'" >> ./.responses
echo "ServiceUser='$ServiceUser'" >> ./.responses
echo "BuildProxy='$BuildProxy'" >> ./.responses
echo "LogLevel='$LogLevel'" >> ./.responses
echo "Kubectl='$Kubectl'" >> ./.responses

echo ""
echo "${orange}Build the application in a docker image and create deployment scripts with specified parameters${nc}"

exec_command="cd /$ApplicationName && go build -o $ApplicationName && /$ApplicationName/$ApplicationName $args"
if ! [ -z "$BuildProxy" ]; then
    exec_command="export HTTP_PROXY='$BuildProxy'; export HTTPS_PROXY='$BuildProxy'; $exec_command"
fi

mkdir -p "$ApplicationDir/.pkg"
echo "Creating the application using: ${orange}$exec_command${nc}"

rm -rf "$ApplicationDir/$ApplicationName"   # make sure that executive does not exists
docker run -it --rm -v "$ApplicationDir:/$ApplicationName" -v "$ApplicationDir/.pkg:/go/pkg" golang:alpine3.12 /bin/sh -c "$exec_command"
rm -rf "$ApplicationDir/$ApplicationName"

echo "Now you may run ${green}deploy.sh${nc} to deploy the application"
