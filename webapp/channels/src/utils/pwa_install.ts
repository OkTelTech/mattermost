// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Helpers for installing Oktel as a web app from the iOS/Android cards on /apps
// (ported from eztac-webapp's platform.ts and pwaInstall.ts).

/** iPhone/iPad, including iPadOS 13+, which reports itself as a Mac. */
export function isIOS(): boolean {
    return (/iPhone|iPod|iPad/).test(navigator.userAgent) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
}

/**
 * A browser that cannot install: an in-app webview (Zalo, Facebook, …), or on
 * iOS anything but Safari, since only Safari can hand a profile to Settings.
 */
export function isInAppBrowser(): boolean {
    const ua = navigator.userAgent;
    if ((/FBAN|FBAV|Instagram|Line\/|MicroMessenger|Zalo|TikTok/).test(ua)) {
        return true;
    }
    return isIOS() && (!(/Safari/).test(ua) || (/CriOS|FxiOS|EdgiOS|OPiOS/).test(ua));
}

/** Launched from the Home Screen icon. */
export function isStandalone(): boolean {
    return (navigator as Navigator & {standalone?: boolean}).standalone === true || window.matchMedia('(display-mode: standalone)').matches;
}

// Chrome fires `beforeinstallprompt` once, early, and it is only usable if
// preventDefault() was called right then. initPwaInstall() therefore runs from
// root.tsx, before the lazily loaded app, and parks the event for /apps.

type BeforeInstallPromptEvent = Event & {
    prompt: () => Promise<void>;
    userChoice: Promise<{outcome: 'accepted' | 'dismissed'}>;
};

let deferred: BeforeInstallPromptEvent | null = null;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((listener) => listener());

export function initPwaInstall() {
    window.addEventListener('beforeinstallprompt', (e) => {
        e.preventDefault();
        deferred = e as BeforeInstallPromptEvent;
        emit();
    });
    window.addEventListener('appinstalled', () => {
        deferred = null;
        emit();
    });
}

export const canInstall = () => deferred !== null;

export function subscribeInstall(listener: () => void) {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

/** Shows Chrome's install dialog. The event is single-use, so it is dropped first. */
export async function promptInstall() {
    const event = deferred;
    if (!event) {
        return false;
    }
    deferred = null;
    emit();
    await event.prompt();
    return (await event.userChoice).outcome === 'accepted';
}
