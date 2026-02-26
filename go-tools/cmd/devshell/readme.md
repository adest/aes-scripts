```
types:
  docker-compose:
    params:
      file: string
    expand:
      children:
        - name: lifecycle
          children:
            - name: up
              command: docker compose -f {{ .file }} up -d
            - name: stop
              command: docker compose -f {{ .file }} stop
```

````
name: stack
uses:
  - docker-compose
file: docker-compose.yml
````

stack
 └─ lifecycle
      ├─ up
      └─ stop