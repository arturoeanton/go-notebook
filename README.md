
# Go-Notebook

Go-Notebook is an app that was developed using go-echo-live-view framework, developed also by us. 

GitHub repository is [here](https://github.com/arturoeanton/go-echo-live-view).

For this project we using https://github.com/cosmos72/gomacro too

## About

Go-Notebook is inspired by Jupyter Project ([link](https://github.com/jupyter/notebook)) in order to document Golang code. 

You can add code and text depends your needs. 

## Run

We will make docker in the future :P

```
git clone https://github.com/arturoeanton/go-notebook.git
cd go-notebook
go run main.go
```

#### Wait 
```
⇨ http server started on [::]:1323
```

### Go to
http://localhost:1323/

## Run on docket

We will make docker in the future :P

```
docker pull arturoeanton/go-notebook
docker run --rm -p 1323:1323 arturoeanton/go-notebook
```
### Go to
http://localhost:1323/


### Change notebooks and snippet folders

#### Docker

```
docker run --rm -p 1323:1323 --volume ./notebooks:/app/notebooks --volume ./snippet:/app/snippet arturoeanton/go-notebook
```

#### Podman 

```
podman run   --rm -p 1323:1323 --volume ./notebooks:/app/notebooks:Z --volume ./snippet:/app/snippet:Z arturoeanton/go-notebook
```

## DEMO

![alt text](https://raw.githubusercontent.com/arturoeanton/go-notebook/main/gonote1.gif)

![alt text](https://raw.githubusercontent.com/arturoeanton/go-notebook/main/gonote2.gif)


## To Do

This project is still in progress, we are working in the following features.
 * Add examples. 
 * Save snippet
 * default.gonote.json convert in folder, and run files in this folder.

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## License
[MIT](https://choosealicense.com/licenses/mit/)
