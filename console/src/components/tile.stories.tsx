import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import Tile, { TileGrid } from "./tile"

const meta: Meta<typeof Tile> = {
    title: "Components/Tile",
    component: Tile,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof Tile>

export const Default: Story = {
    render: () => (
        <Tile title="Email / Password">
            Authenticate users with a classic email and password combination.
        </Tile>
    ),
}

export const Selected: Story = {
    render: () => (
        <Tile title="Social Login" selected>
            Allow users to sign in with their existing social accounts.
        </Tile>
    ),
}

export const Clickable: Story = {
    render: () => {
        const [selected, setSelected] = useState<string | null>(null)
        return (
            <TileGrid numColumns={2}>
                {["Email", "Google", "GitHub", "SAML"].map((option) => (
                    <Tile
                        key={option}
                        title={option}
                        selected={selected === option}
                        onClick={() => setSelected(option)}
                    >
                        Connect with {option}.
                    </Tile>
                ))}
            </TileGrid>
        )
    },
}

export const LargeSize: Story = {
    render: () => (
        <Tile title="Pro Plan" size="large">
            Everything in Starter, plus advanced analytics, priority support, and unlimited
            integrations.
        </Tile>
    ),
}

export const Grid: Story = {
    render: () => (
        <TileGrid numColumns={3}>
            <Tile title="Email / Password">Classic authentication method.</Tile>
            <Tile title="Google OAuth" selected>
                Sign in with Google.
            </Tile>
            <Tile title="SAML SSO">Enterprise single sign-on.</Tile>
        </TileGrid>
    ),
}
