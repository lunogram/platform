import type { Meta, StoryObj } from "@storybook/react"
import CodeExample from "./code-example"

const meta: Meta<typeof CodeExample> = {
    title: "Components/CodeExample",
    component: CodeExample,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof CodeExample>

const jsonCode = `{
  "id": "usr_01j9x2k3m4n5p6q7",
  "email": "alice@example.com",
  "name": "Alice Smith",
  "created_at": "2024-01-15T10:30:00Z",
  "metadata": {
    "plan": "pro",
    "seats": 10
  }
}`

const jsCode = `import { createClient } from "@myapp/sdk"

const client = createClient({
  projectId: "proj_abc123",
  secretKey: process.env.SECRET_KEY,
})

const user = await client.users.get("usr_01j9x2k3m4n5p6q7")
console.log(user.email)`

export const JsonExample: Story = {
    render: () => (
        <div className="max-w-xl">
            <CodeExample
                code={jsonCode}
                title="User Object"
                description="The user record returned from the API."
            />
        </div>
    ),
}

export const JavaScriptExample: Story = {
    render: () => (
        <div className="max-w-xl">
            <CodeExample
                code={jsCode}
                title="Fetch a User"
                description="Use the SDK to retrieve a user by ID."
            />
        </div>
    ),
}

export const NoTitle: Story = {
    render: () => (
        <div className="max-w-xl">
            <CodeExample code={jsonCode} />
        </div>
    ),
}
