docker_image_name="massh-ssh-test"

if  [[ $(docker ps -a -q --filter ancestor=$docker_image_name --format="{{.ID}}") ]]; then
  echo "############ REMOVE CONTAINER ############"
  # Remove existing containers using this image.
  docker rm $(docker stop $(docker ps -a -q --filter ancestor=$docker_image_name --format="{{.ID}}"))
fi

echo "############ BUILD CONTAINER ############"
docker build -t $docker_image_name .

echo "############ RUN CONTAINER ############"
docker run -d -p 22:22 $docker_image_name
