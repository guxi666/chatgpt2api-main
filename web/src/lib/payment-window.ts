export function reservePaymentWindow() {
  if (typeof window === "undefined") {
    return null;
  }
  const paymentWindow = window.open("about:blank", "_blank");
  if (!paymentWindow) {
    return null;
  }
  paymentWindow.document.title = "支付订单创建中";
  paymentWindow.document.body.innerHTML =
    '<div style="font-family: system-ui, sans-serif; padding: 32px; color: #111827;">支付订单创建中，请稍候...</div>';
  return paymentWindow;
}

export function openPaymentURL(paymentWindow: Window | null, payURL: string) {
  if (paymentWindow && !paymentWindow.closed) {
    paymentWindow.opener = null;
    paymentWindow.location.href = payURL;
    return;
  }
  window.location.href = payURL;
}

export function closePaymentWindow(paymentWindow: Window | null) {
  if (paymentWindow && !paymentWindow.closed) {
    paymentWindow.close();
  }
}
