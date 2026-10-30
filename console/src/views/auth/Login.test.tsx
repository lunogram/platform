import { render, screen } from "@testing-library/react"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import Login from "./Login"

const translate = vi.hoisted(() => (key: string) => key)

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: translate }),
}))
vi.mock("@clerk/clerk-react", () => ({ SignIn: () => null }))
vi.mock("@/api", () => ({
    default: { auth: { methods: vi.fn(), ssoProviders: vi.fn(), basicAuth: vi.fn() } },
}))

function renderLogin() {
    const router = createMemoryRouter([{ path: "/login", element: <Login /> }], {
        initialEntries: ["/login"],
    })
    render(<RouterProvider router={router} />)
}

describe("login", () => {
    afterEach(() => {
        vi.clearAllMocks()
    })

    it("says the sign-in methods failed to load instead of spinning", async () => {
        vi.mocked(api.auth.methods).mockRejectedValue(new Error("network down"))

        renderLogin()

        expect(await screen.findByText("login_methods_error")).toBeTruthy()
        expect(screen.queryByText("loading")).toBeNull()
    })

    it("says so when the deployment offers no supported sign-in method", async () => {
        vi.mocked(api.auth.methods).mockResolvedValue([])

        renderLogin()

        expect(await screen.findByText("login_methods_error")).toBeTruthy()
        expect(screen.queryByText("loading")).toBeNull()
    })

    it("shows the password form when basic auth is offered", async () => {
        vi.mocked(api.auth.methods).mockResolvedValue(["basic"])

        renderLogin()

        expect(await screen.findByText("login_basic_instructions")).toBeTruthy()
    })
})
