### 1. Database Table Design

First, create an `employees` table in MySQL to store employee information:

```sql
CREATE DATABASE employee_db;

USE employee_db;

CREATE TABLE employees (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    position VARCHAR(100),
    department VARCHAR(100),
    bio TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

### 2. Backend Implementation (Gin + MySQL)

[main.go](./backend/main.go)

### 3. Frontend API Design

Here, two API endpoints are defined:

1. **Get Employee Info (GET)**: `/employee/:id`
   - Used to retrieve detailed information about a specific employee.

2. **Update Employee Info (PUT)**: `/employee/:id`
   - Used to update employee information.

### 4. Database Connection Configuration

Replace the `dsn` with your MySQL username and password in the connection string.

### 5. Running the Application

1. Start the MySQL database.
2. Execute the SQL statements to create the database and table.
3. Run the Go application:
   ```bash
   git clone https://github.com/kafkaqin/interview.git;
   cd interview/104_employee_page
   docker-compose up -d --build
   ```
4. The server will run on `localhost:8080`. You can test the following APIs:
   - `GET /employee/1` to get the employee information with ID 1.
        ```
        {
            "ID": 1,
            "CreatedAt": "2024-10-13T16:15:54.813+08:00",
            "UpdatedAt": "2024-10-13T16:15:54.813+08:00",
            "DeletedAt": null,
            "name": "John Doe",
            "email": "john.doe@example.com",
            "position": "Software Engineer",
            "department": "",
            "bio": "Golang enthusiast"
        }
        ```
     - `GET /employee/email/:email` to get the employee information with email.
        ```
        {
            "ID": 1,
            "CreatedAt": "2024-10-13T16:15:54.813+08:00",
            "UpdatedAt": "2024-10-13T16:15:54.813+08:00",
            "DeletedAt": null,
            "name": "John Doe",
            "email": "john.doe@example.com",
            "position": "Software Engineer",
            "department": "",
            "bio": "Golang enthusiast"
        }
        ```
    
   - `POST /employee/1` to update the employee information with ID 1.
        ```
        {
            "ID": 1,
            "CreatedAt": "2024-10-13T16:15:54.813+08:00",
            "UpdatedAt": "2024-10-13T16:15:54.813+08:00",
            "DeletedAt": null,
            "name": "John Doe 111",
            "email": "john.doe@example.com",
            "position": "Software Engineer",
            "department": "",
            "bio": "Golang enthusiast"
        }
        ```
   - `POST /employee` to create new employee information.
        ```
        {
        "name": "John Doe",
        "email": "john.doe@example.com",
        "position": "Software Engineer",
        "bio": "Golang enthusiast"
        }
        ```

5. Stop 
    ```
     cd interview/104_employee_page
     docker-compose down
   ```
### Conclusion

This code provides basic backend API support for an employee page and can be extended and integrated with a frontend. If you need a frontend implementation, you can use TypeScript, React, and Tailwind CSS to build the user interface.