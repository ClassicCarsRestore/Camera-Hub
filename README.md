# CameraHub
Camera Hub application that uses the Charter of Turin Monitor API to provide its users access and control over remote IP cameras.

## Configuration

### Edit file authentication.go

Change the secret value to match the one in the Charter of Turin Monitor.

Format:

```bash
const secret = "yoursecret"
```

### Edit file config.json

Change the domain, ports and credentials of the cameras.

Format:

```bash
{
  "server": {
    "http_port": ":8083"
  },
  "streams": {
    "Camera1": {
      "on_demand": true,
      "domain": "1.1.1.1",
      "rtspport": "554",
      "httpport": "80",
      "username": "username",
      "password": "password"
    }
  }
}
```
