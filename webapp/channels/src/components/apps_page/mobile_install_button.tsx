// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useSyncExternalStore} from 'react';
import {defineMessages, useIntl} from 'react-intl';

import {GenericModal} from '@mattermost/components';

import {canInstall, isIOS, isInAppBrowser, isStandalone, promptInstall, subscribeInstall} from 'utils/pwa_install';
import {getBasePath, getSiteURL} from 'utils/url';
import {isAndroid} from 'utils/user_agent';
import {copyToClipboard} from 'utils/utils';

import './mobile_install_button.scss';

export type MobilePlatform = 'ios' | 'android';

const iosSteps = defineMessages({
    step1: {id: 'apps_page.mobile_install.ios_step1', defaultMessage: 'Tap Allow when Safari asks to download a configuration profile.'},
    step2: {id: 'apps_page.mobile_install.ios_step2', defaultMessage: 'Open Settings. "Profile Downloaded" appears near the top. On older iOS go to General > VPN & Device Management.'},
    step3: {id: 'apps_page.mobile_install.ios_step3', defaultMessage: 'Tap Install, enter your passcode, then tap Install again to confirm.'},
    step4: {id: 'apps_page.mobile_install.ios_step4', defaultMessage: 'The Oktel icon is now on your Home Screen. Open it from there.'},
});

// Why the modal is open: the rest of the iOS install happens in Settings, Android
// waits for Chrome to offer its install prompt, a browser that cannot install
// has to hand over to a real one, or a computer continues on the phone via QR.
type ModalView = 'ios_steps' | 'android_install' | 'open_in_browser' | 'qr';

type Props = {
    platform: MobilePlatform;
}

/**
 * Download button of the iOS and Android cards on /apps. It installs Oktel as a
 * web app straight away (flow ported from eztac-webapp):
 *  - iOS: opens the Web Clip profile in src/install/Oktel.mobileconfig.
 *  - Android: shows Chrome's install dialog (see utils/pwa_install).
 * Nothing is installed on a computer.
 */
export default function MobileInstallButton({platform}: Props) {
    const {formatMessage} = useIntl();
    const [modalView, setModalView] = useState<ModalView | null>(null);
    const [installedNow, setInstalledNow] = useState(false);
    const [copied, setCopied] = useState(false);

    const installable = useSyncExternalStore(subscribeInstall, canInstall);

    const onThisPlatform = platform === 'ios' ? isIOS() : isAndroid();
    const installed = onThisPlatform && (installedNow || isStandalone());
    const pageUrl = `${getSiteURL()}/apps`;
    const values = {
        device: platform === 'ios' ? 'iPhone/iPad' : 'Android',
        browser: platform === 'ios' ? 'Safari' : 'Chrome',
    };

    // A plain navigation makes Safari offer the profile to Settings; a download
    // link would only save it to Files.
    const installProfile = () => {
        window.location.href = `${getBasePath()}/static/Oktel.mobileconfig`;
        setModalView('ios_steps');
    };

    const installWithPrompt = async () => {
        if (await promptInstall()) {
            setInstalledNow(true);
            setModalView(null);
        }
    };

    const handleClick = () => {
        if (!onThisPlatform) {
            setModalView('qr');
        } else if (isInAppBrowser()) {
            setModalView('open_in_browser');
        } else if (platform === 'ios') {
            installProfile();
        } else if (installable) {
            installWithPrompt();
        } else {
            setModalView('android_install');
        }
    };

    const copyLink = () => {
        copyToClipboard(pageUrl);
        setCopied(true);
    };

    let modalBody;
    switch (modalView) {
    case 'ios_steps':
        modalBody = (
            <>
                <ol className='mobile-install__steps'>
                    {Object.values(iosSteps).map((step) => (
                        <li key={step.id}>{formatMessage(step)}</li>
                    ))}
                </ol>
                <div className='mobile-install__warn'>
                    {formatMessage({id: 'apps_page.mobile_install.ios_unverified', defaultMessage: 'Settings marks the profile "Unverified". That is normal for this kind of profile and is safe to continue past.'})}
                </div>
                <button
                    className='btn btn-tertiary'
                    onClick={installProfile}
                >
                    {formatMessage({id: 'apps_page.mobile_install.download_again', defaultMessage: 'Download again'})}
                </button>
            </>
        );
        break;
    case 'android_install':
        // Chrome offers the prompt only after a tap and 30 seconds on the site.
        modalBody = (
            <>
                <p>{formatMessage({id: 'apps_page.mobile_install.android_waiting', defaultMessage: 'The button turns on once Chrome is ready to install Oktel, usually within 30 seconds. If it stays off, open this page in Chrome.'})}</p>
                <button
                    className='btn btn-primary'
                    onClick={installWithPrompt}
                    disabled={!installable}
                >
                    {formatMessage({id: 'apps_page.mobile_install.install_app', defaultMessage: 'Install app'})}
                </button>
            </>
        );
        break;
    case 'open_in_browser':
        modalBody = (
            <>
                <p>{formatMessage({id: 'apps_page.mobile_install.open_in_browser', defaultMessage: 'Installing needs {browser}. Copy the link below, open it in {browser}, then tap Download again.'}, values)}</p>
                <div className='mobile-install__url'>{pageUrl}</div>
                <button
                    className='btn btn-primary'
                    onClick={copyLink}
                >
                    {copied ? formatMessage({id: 'apps_page.mobile_install.copied', defaultMessage: 'Link copied'}) : formatMessage({id: 'apps_page.mobile_install.copy_link', defaultMessage: 'Copy link'})}
                </button>
            </>
        );
        break;
    case 'qr':
        modalBody = (
            <>
                <img
                    className='mobile-install__qr'
                    src={`${getBasePath()}/static/install-qr.svg`}
                    alt={formatMessage({id: 'apps_page.mobile_install.qr_alt', defaultMessage: 'QR code to the Oktel apps page'})}
                />
                <p>{formatMessage({id: 'apps_page.mobile_install.qr_hint', defaultMessage: 'Scan this code with your {device} to open this page in {browser}, then tap Download.'}, values)}</p>
            </>
        );
        break;
    }

    return (
        <>
            <button
                className='apps-page-card-button'
                onClick={handleClick}
                disabled={installed}
            >
                {installed ? formatMessage({id: 'apps_page.mobile_install.installed', defaultMessage: 'Installed on this device'}) : formatMessage({id: 'apps_page.download', defaultMessage: 'Download'})}
            </button>
            {modalView && (
                <GenericModal
                    id='mobileInstallModal'
                    show={true}
                    onHide={() => setModalView(null)}
                    modalHeaderText={formatMessage({id: 'apps_page.mobile_install.title', defaultMessage: 'Install on {device}'}, values)}
                    compassDesign={true}
                >
                    <div className='mobile-install'>{modalBody}</div>
                </GenericModal>
            )}
        </>
    );
}
