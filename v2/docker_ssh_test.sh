docker_image_name="massh-ssh-test"

echo "############ BUILD CONTAINER ############"
docker build -t $docker_image_name .

echo "############ RUN CONTAINER ############"
docker run -d -p 22:22 $docker_image_name

echo "############ GO TEST ############"
go test ./...

echo "############ REMOVE CONTAINER ############"
# Remove all containers using the test image.
docker rm $(docker stop $(docker ps -a -q --filter ancestor=$docker_image_name --format="{{.ID}}"))
