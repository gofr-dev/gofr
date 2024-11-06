export const navigation = [
    {
        title: 'Quick Start Guide',
        desc: "Get started with GoFR through our Quick Start Guide. Learn to build scalable applications with easy-to-follow instructions on server setup, database connections, configuration management, and more. Boost your productivity and streamline your development process.",
        links: [
            {
                title: 'Hello Server',
                href: '/docs/quick-start/introduction' ,
                desc: "Getting started with how to write a server using GoFR with basic examples and explanations. Boost your productivity with efficient coding practices and learn to build scalable applications quickly."},
            {
                title: 'Configuration',
                href: '/docs/quick-start/configuration',
                desc: "Set up environment variables, manage settings, and streamline your development process."
            },
            {
                title: 'Connecting Redis',
                href: '/docs/quick-start/connecting-redis',
                desc: "Discover how to connect your GoFR application to Redis for fast in-memory data storage."
            },
            {
                title: 'Connecting MySQL',
                href: '/docs/quick-start/connecting-mysql',
                desc: "Step-by-step guide on integrating MySQL with your GoFR application. With managed database connections and new methods for increasing your productivity."
            },
            {
                title: 'Observability',
                href: '/docs/quick-start/observability',
                desc: "Inbuilt logging, tracing, and metrics to enhance reliability and performance."
            },
            {
                title: 'Adding REST Handlers',
                href: '/docs/quick-start/add-rest-handlers',
                desc: "Fastest way to create CRUD APIs by just providing the entity."
            }
        ],
    },
    {
        title: 'Advanced Guide',
        links: [
            {
                title: "Scheduling Cron Jobs",
                href: "/docs/advanced-guide/using-cron",
                desc: "Learn how to schedule and manage cron jobs in your application for automated tasks and background processes with GoFr's CRON job management."
            },
            {
                title: 'Overriding Default',
                href: '/docs/advanced-guide/overriding-default',
                desc: "Understand how to override default configurations and behaviors in GoFR to tailor framework to your specific needs."
            },
            {
                title: 'Remote Log Level Change',
                href: '/docs/advanced-guide/remote-log-level-change',
                desc: "Discover how to dynamically change log levels remotely, enabling you to adjust logging verbosity without redeploying your application."
            },
            {
                title: 'Publishing Custom Metrics',
                href: '/docs/advanced-guide/publishing-custom-metrics',
                desc: "Explore methods for publishing custom metrics to monitor your application's performance and gain valuable insights."
            },
            {
                title: 'Custom Spans in Tracing',
                href: '/docs/advanced-guide/custom-spans-in-tracing',
                desc: "Learn to create custom spans for tracing to enhance observability and analyze the performance of your services."
            },
            {
                title: 'Adding Custom Middleware',
                href: '/docs/advanced-guide/middlewares',
                desc: "Learn how to add custom middleware to your GoFR application for enhanced functionality and request processing."
            },
            {
                title: 'HTTP Communication',
                href: '/docs/advanced-guide/http-communication',
                desc: "Get familiar with making HTTP requests and handling responses within your GoFR application to facilitate seamless communication."
            },
            {
                title: 'HTTP Authentication',
                href: '/docs/advanced-guide/http-authentication',
                desc: "Implement various HTTP authentication methods to secure your GoFR application and protect sensitive endpoints."
            },
            {
                title: 'Circuit Breaker Support',
                href: '/docs/advanced-guide/circuit-breaker',
                desc: "Understand how to implement circuit breaker patterns to enhance the resilience of your services against failures."
            },
            {
                title: 'Monitoring Service Health',
                href: '/docs/advanced-guide/monitoring-service-health',
                desc: "Learn to monitor the health of your services effectively, ensuring optimal performance and quick issue resolution."
            },
            {
                title: 'Handling Data Migrations',
                href: '/docs/advanced-guide/handling-data-migrations',
                desc: "Explore strategies for managing data migrations within your GoFR application to ensure smooth transitions and data integrity."
            },
            {
                title: 'Writing gRPC Server',
                href: '/docs/advanced-guide/grpc',
                desc: "Step-by-step guide on writing a gRPC server in GoFR to facilitate efficient communication between services."
            },
            {
                title: 'Using Pub/Sub',
                href: '/docs/advanced-guide/using-publisher-subscriber',
                desc: "Discover how to gofr seamlessly allows to integrate different Pub/Sub systems in your application for effective messaging and event-driven architectures."
            },
            {
                title: 'Injecting Databases',
                href: '/docs/advanced-guide/injecting-databases-drivers',
                desc: "Learn how to inject database drivers into your GoFR application for seamless data management and operations."
            },
            {
                title: 'Key Value Store',
                href: '/docs/advanced-guide/key-value-store',
                desc: "Explore how to implement and manage a key-value store in your GoFR application for fast and efficient data retrieval."
            },
            {
                title: 'Dealing with SQL',
                href: '/docs/advanced-guide/dealing-with-sql',
                desc: "Get insights into best practices for working with SQL databases in GoFR, including query optimization and error handling."
            },
            {
                title: 'Automatic SwaggerUI Rendering',
                href: '/docs/advanced-guide/swagger-documentation',
                desc: "Learn how to automatically render SwaggerUI documentation for your GoFR APIs, improving discoverability and usability."
            },
            {
                title: 'Error Handling',
                href: '/docs/advanced-guide/gofr-errors',
                desc: "Understand error handling mechanisms in GoFR to ensure robust applications and improved user experience."
            },
            {
                title: 'Handling File',
                href: '/docs/advanced-guide/handling-file',
                desc: "Explore how GoFR enables efficient file handling by abstracting remote and local filestore providers in your Go application. Learn to manage file uploads, downloads, and storage seamlessly, enhancing your application's capability to work with diverse data sources."
            },
            {
                title: 'WebSockets',
                href: '/docs/advanced-guide/websocket',
                desc: "Explore how gofr eases the process of WebSocket communication in your Golang application for real-time data exchange."
            }
        ],
    },
    {
        title: 'References',
        links: [
            {
                title: 'Context',
                href: '/docs/references/context',
                desc: "Discover the GoFR context, an injected object that simplifies request-specific data handling for HTTP, gRPC, and Pub/Sub calls. Learn how it extends Go's context, providing easy access to dependencies like databases, loggers, and HTTP clients. Explore features for reading HTTP requests, binding data, and accessing query and path parameters efficiently, all while reducing application complexity."
            },
            {
                title: 'Configs',
                href: '/docs/references/configs',
                desc: "Learn how to manage configuration settings in your GoFR applications, including default values for environment variables. This section provides a comprehensive list of all available configurations to streamline your setup."
            },
            {
                title: 'Testing',
                href: '/docs/references/testing',
                desc: "GoFR provides a centralized collection of mocks to facilitate writing effective unit tests. Explore testing strategies and tools for GoFR applications, ensuring your code is robust, reliable, and maintainable."
            }
        ],
    },
]