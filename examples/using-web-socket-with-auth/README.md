# WebSocket with Authentication Example

This GoFr example demonstrates how to implement WebSocket connections with authentication middleware. It shows how to:

1. Set up Basic Authentication for your GoFr application with a custom validator
2. Use WebSockets with authenticated connections
3. Handle messages from authenticated clients
4. Track active WebSocket connections
5. Extract username from authentication credentials

## Features

- **Authenticated WebSocket Connections**: Only authenticated users can establish WebSocket connections
- **User Tracking**: The example keeps track of connected users and provides an endpoint to list them
- **Custom Authentication**: Uses a custom validator function to authenticate users
- **Chat-like Functionality**: Demonstrates a simple chat application where messages include usernames

## How Authentication Works with WebSockets

WebSockets start as HTTP connections that are then upgraded to WebSocket protocol. In GoFr, authentication middleware is applied during the initial HTTP handshake, before the connection is upgraded to a WebSocket.

The authentication flow works as follows:

1. Client sends an HTTP request with authentication credentials (e.g., Basic Auth header)
2. GoFr's authentication middleware validates the credentials using the custom validator
3. If authentication succeeds, the connection is upgraded to WebSocket
4. If authentication fails, a 401 Unauthorized response is returned, and the WebSocket connection is not established

This example uses Basic Authentication for simplicity, but the same principle applies to other authentication methods like API Key, OAuth, or custom authentication.

## Running the Example

To run the example, use the following command:

```console
go run main.go
```

## Testing the WebSocket Connection

You can test the WebSocket connection using tools like [websocat](https://github.com/vi/websocat) or browser-based WebSocket clients.

### Using websocat with Basic Auth

```console
websocat ws://localhost:8000/ws -H="Authorization: Basic dXNlcjE6cGFzc3dvcmQx"
```

The Basic Auth header `dXNlcjE6cGFzc3dvcmQx` is the base64-encoded string of `user1:password1`.

### Using curl to check active users

You can use curl to check the list of active users:

```console
curl -u user1:password1 http://localhost:8000/users
```

This will return a JSON response with the list of currently connected users.

### Using JavaScript in a Browser

```javascript
// Function to create a WebSocket with authentication
function createAuthenticatedWebSocket(url, username, password) {
    // Create a custom WebSocket object that includes authentication
    return new Promise((resolve, reject) => {
        // Create the WebSocket connection
        const socket = new WebSocket(url);

        // Add authentication headers to the connection
        // Note: This is a workaround as browsers don't allow setting headers directly
        // In a real application, you would use a token-based approach

        // Connection opened
        socket.addEventListener('open', (event) => {
            console.log('Connected to WebSocket server');
            resolve(socket);
        });

        // Connection error
        socket.addEventListener('error', (event) => {
            console.error('WebSocket connection error:', event);
            reject(event);
        });
    });
}

// Usage example
async function connectToChat() {
    try {
        // Connect to the WebSocket server
        // Note: In a real application, you would need to handle authentication differently
        // as browsers don't allow setting custom headers for WebSockets
        const socket = await createAuthenticatedWebSocket('ws://localhost:8000/ws', 'user1', 'password1');

        // Send a message
        socket.send(JSON.stringify({content: 'Hello from browser!'}));

        // Listen for messages
        socket.addEventListener('message', (event) => {
            console.log('Message from server:', event.data);
            // Display the message in the UI
            const messagesDiv = document.getElementById('messages');
            messagesDiv.innerHTML += `<div>${event.data}</div>`;
        });

        // Set up UI for sending messages
        document.getElementById('send-button').addEventListener('click', () => {
            const messageInput = document.getElementById('message-input');
            const message = messageInput.value;
            if (message) {
                socket.send(JSON.stringify({content: message}));
                messageInput.value = '';
            }
        });
    } catch (error) {
        console.error('Failed to connect:', error);
    }
}

// Start the connection
connectToChat();
```

**Note**: Browser WebSocket API doesn't allow setting custom headers directly. In a real application, you would typically use a token-based approach where the token is obtained via a separate authenticated HTTP request and then included in the WebSocket URL or in the first message sent after connection.

## Security Considerations

In a production environment, consider these security best practices:

1. Use HTTPS/WSS instead of HTTP/WS to encrypt the connection
2. Implement token-based authentication (JWT) instead of Basic Auth
3. Validate user permissions for specific WebSocket actions
4. Implement rate limiting to prevent abuse
5. Sanitize and validate all incoming WebSocket messages
6. Store user credentials securely (e.g., hashed passwords in a database)
7. Implement proper session management and token expiration

## Implementation Details

This example demonstrates several key concepts:

1. **Authentication Middleware**: GoFr's authentication middleware is applied before the WebSocket connection is established, ensuring only authenticated users can connect.

2. **Custom Validator**: The example uses a custom validator function to authenticate users, which could be extended to validate against a database.

3. **Username Extraction**: The example extracts the username from the Basic Auth header to identify the user in the WebSocket connection.

4. **Connection Tracking**: The example keeps track of active connections and provides an endpoint to list them.

5. **Continuous Message Handling**: The WebSocket handler uses a loop to continuously process incoming messages until the connection is closed.

## Additional Resources

- [GoFr Documentation](https://gofr.dev)
- [WebSocket Protocol](https://tools.ietf.org/html/rfc6455)
- [HTTP Authentication](https://developer.mozilla.org/en-US/docs/Web/HTTP/Authentication)
