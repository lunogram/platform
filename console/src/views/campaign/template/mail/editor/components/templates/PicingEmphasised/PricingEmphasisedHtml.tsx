export function PricingEmphasisedHtml() {
    return (
        <div
            style={{
                backgroundColor: "#ffffff",
                padding: "24px",
                fontFamily: "sans-serif",
                maxWidth: "600px",
                margin: "0 auto",
                borderRadius: "8px",
            }}
        >
            {/* Header */}
            <div style={{ textAlign: "center", marginBottom: "32px" }}>
                <h1
                    style={{
                        fontSize: "24px",
                        fontWeight: 600,
                        color: "#101828",
                        marginBottom: "12px",
                    }}
                >
                    Choose the right plan for you
                </h1>
                <p style={{ fontSize: "14px", color: "#6b7280", lineHeight: 1.5 }}>
                    Choose an affordable plan with top features to engage audiences, build loyalty,
                    and boost sales.
                </p>
            </div>

            {/* Hobby Plan Card */}
            <div
                style={{
                    border: "1px solid #d1d5db",
                    borderRadius: "8px",
                    padding: "24px",
                    marginBottom: "24px",
                    backgroundColor: "#ffffff",
                }}
            >
                <span
                    style={{
                        fontSize: "14px",
                        fontWeight: 600,
                        color: "#4f46e5",
                        display: "block",
                        marginBottom: "16px",
                    }}
                >
                    Hobby
                </span>
                <div style={{ marginBottom: "12px" }}>
                    <span style={{ fontSize: "28px", fontWeight: 700, color: "#101828" }}>$29</span>
                    <span style={{ fontSize: "14px", color: "#6b7280" }}> / month</span>
                </div>
                <p style={{ color: "#6b7280", fontSize: "14px", marginBottom: "24px" }}>
                    The perfect plan for getting started.
                </p>
                <a
                    href="#"
                    style={{
                        display: "block",
                        textAlign: "center",
                        backgroundColor: "#4f46e5",
                        color: "#ffffff",
                        padding: "12px",
                        borderRadius: "8px",
                        textDecoration: "none",
                        fontWeight: 600,
                    }}
                >
                    Get started today
                </a>
            </div>

            {/* Enterprise Plan Card (Highlighted) */}
            <div
                style={{
                    backgroundColor: "#101828",
                    borderRadius: "8px",
                    padding: "24px",
                    marginBottom: "32px",
                }}
            >
                <span
                    style={{
                        fontSize: "14px",
                        fontWeight: 600,
                        color: "#7c86ff",
                        display: "block",
                        marginBottom: "16px",
                    }}
                >
                    Enterprise
                </span>
                <div style={{ marginBottom: "12px" }}>
                    <span style={{ fontSize: "28px", fontWeight: 700, color: "#ffffff" }}>$99</span>
                    <span style={{ fontSize: "14px", color: "#d1d5db" }}> / month</span>
                </div>
                <p style={{ color: "#d1d5db", fontSize: "14px", marginBottom: "24px" }}>
                    Dedicated support and enterprise ready.
                </p>
                <a
                    href="#"
                    style={{
                        display: "block",
                        textAlign: "center",
                        backgroundColor: "#4f46e5",
                        color: "#ffffff",
                        padding: "12px",
                        borderRadius: "8px",
                        textDecoration: "none",
                        fontWeight: 600,
                    }}
                >
                    Get started today
                </a>
            </div>

            <hr
                style={{
                    border: 0,
                    borderTop: "1px solid #d1d5db",
                    marginBottom: "16px",
                }}
            />

            {/* Footer */}
            <p
                style={{
                    fontSize: "12px",
                    color: "#6b7280",
                    textAlign: "center",
                    marginTop: "32px",
                }}
            >
                Customer Experience Research Team
            </p>
        </div>
    )
}
