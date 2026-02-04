// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import { useIntl } from 'react-intl';
import { useSelector } from 'react-redux';

import { GenericModal } from '@mattermost/components';
import { getConfig } from 'mattermost-redux/selectors/entities/general';

import type { CustomizeHeaderType } from 'components/header_footer_route/header_footer_route';

import './apps_page.scss';

import macosTerminal from 'images/macos-terminal.jpg';
import macosTerminalCommands from 'images/macos-terminal-commands.png';

type AppsPageProps = {
    onCustomizeHeader?: CustomizeHeaderType;
}

type AppDownloadCard = {
    id: string;
    title: string;
    description: string;
    iconClasses: string[];
    downloadUrl: string;
    platforms: string[];
}

const AppsPage = ({ onCustomizeHeader }: AppsPageProps) => {
    const { formatMessage } = useIntl();
    const [showMacInstructionModal, setShowMacInstructionModal] = React.useState(false);
    const config = useSelector(getConfig);

    const defaultDesktopDownloadLink = config?.AppDownloadLink || 'https://oktel.io/download#desktop';
    const windowsDownloadLink = config?.WindowsAppDownloadLink || defaultDesktopDownloadLink;
    const linuxDownloadLink = config?.LinuxAppDownloadLink || defaultDesktopDownloadLink;
    const macIntelDownloadLink = config?.MacosIntelAppDownloadLink || defaultDesktopDownloadLink;
    const macMDownloadLink = config?.MacosMAppDownloadLink || defaultDesktopDownloadLink;
    const iosDownloadLink = config?.IosAppDownloadLink || 'https://oktel.io/pl/ios-app/';
    const androidDownloadLink = config?.AndroidAppDownloadLink || 'https://oktel.io/pl/android-app/';

    React.useEffect(() => {
        if (onCustomizeHeader) {
            onCustomizeHeader({
                onBackButtonClick: undefined,
            });
        }
    }, [onCustomizeHeader]);

    const appCards: AppDownloadCard[] = [
        {
            id: 'windows',
            title: formatMessage({ id: 'apps_page.windows.title', defaultMessage: 'Windows' }),
            description: formatMessage({ id: 'apps_page.windows.description', defaultMessage: 'Windows 10+' }),
            iconClasses: ['fa fa-windows'],
            downloadUrl: windowsDownloadLink,
            platforms: ['Windows'],
        },
        {
            id: 'linux',
            title: formatMessage({ id: 'apps_page.linux.title', defaultMessage: 'Linux' }),
            description: formatMessage({ id: 'apps_page.linux.description', defaultMessage: 'Ubuntu, Debian, Fedora' }),
            iconClasses: ['fa fa-linux'],
            downloadUrl: linuxDownloadLink,
            platforms: ['Linux'],
        },
        {
            id: 'macos-intel',
            title: formatMessage({ id: 'apps_page.macos_intel.title', defaultMessage: 'macOS Intel' }),
            description: formatMessage({ id: 'apps_page.macos_intel.description', defaultMessage: 'macOS 11+' }),
            iconClasses: ['fa fa-apple'],
            downloadUrl: macIntelDownloadLink,
            platforms: ['macOS Intel'],
        },
        {
            id: 'macos-m',
            title: formatMessage({ id: 'apps_page.macos_m.title', defaultMessage: 'macOS Apple Silicon' }),
            description: formatMessage({ id: 'apps_page.macos_m.description', defaultMessage: 'M1, M2, M3 chips' }),
            iconClasses: ['fa fa-apple'],
            downloadUrl: macMDownloadLink,
            platforms: ['macOS M series'],
        },
        {
            id: 'ios',
            title: formatMessage({ id: 'apps_page.ios.title', defaultMessage: 'iOS' }),
            description: formatMessage({ id: 'apps_page.ios.description', defaultMessage: 'iPhone & iPad' }),
            iconClasses: ['fa fa-apple'],
            downloadUrl: iosDownloadLink,
            platforms: ['iOS'],
        },
        {
            id: 'android',
            title: formatMessage({ id: 'apps_page.android.title', defaultMessage: 'Android' }),
            description: formatMessage({ id: 'apps_page.android.description', defaultMessage: 'Phones & tablets' }),
            iconClasses: ['fa fa-android'],
            downloadUrl: androidDownloadLink,
            platforms: ['Android'],
        },
    ];

    return (
        <div className='apps-page-body'>
            <div className='apps-page-body-content'>
                <div className='apps-page-header'>
                    <h1 className='apps-page-title'>
                        {formatMessage({ id: 'apps_page.title', defaultMessage: 'Download Apps' })}
                    </h1>
                    <p className='apps-page-subtitle'>
                        {formatMessage({
                            id: 'apps_page.subtitle',
                            defaultMessage: 'Get the desktop and mobile apps for the best experience'
                        })}
                    </p>
                </div>
                <div className='apps-page-cards-grid'>
                    {appCards.map((app) => (
                        <div
                            key={app.id}
                            className='apps-page-card'
                        >
                            <div className='apps-page-card-icon'>
                                {app.iconClasses.map((iconClass, index) => (
                                    <i
                                        key={`${app.id}-icon-${index}`}
                                        className={iconClass}
                                        aria-hidden='true'
                                    />
                                ))}
                            </div>
                            <div className='apps-page-card-content'>
                                <h3 className='apps-page-card-title'>{app.title}</h3>
                                <p className='apps-page-card-description'>{app.description}</p>
                                <div className='apps-page-card-platforms'>
                                    {app.platforms.map((platform) => (
                                        <span
                                            key={platform}
                                            className='apps-page-card-platform-tag'
                                        >
                                            {platform}
                                        </span>
                                    ))}
                                </div>
                            </div>
                            <div className='apps-page-card-actions'>
                                <a
                                    href={app.downloadUrl}
                                    className='apps-page-card-button'
                                    target='_blank'
                                    rel='noopener noreferrer'
                                >
                                    {formatMessage({ id: 'apps_page.download', defaultMessage: 'Download' })}
                                </a>
                                {(app.id === 'macos-intel' || app.id === 'macos-m') && (
                                    <button
                                        className='apps-page-card-button apps-page-card-button--secondary'
                                        onClick={(e) => {
                                            e.preventDefault();
                                            setShowMacInstructionModal(true);
                                        }}
                                    >
                                        {formatMessage({ id: 'apps_page.mac_instruction_modal.button', defaultMessage: 'Installation Guide' })}
                                    </button>
                                )}
                            </div>
                        </div>
                    ))}
                </div>
            </div>
            {showMacInstructionModal && (
                <GenericModal
                    id='macInstructionModal'
                    className='mac-instruction-modal'
                    show={showMacInstructionModal}
                    onHide={() => setShowMacInstructionModal(false)}
                    modalHeaderText={formatMessage({ id: 'apps_page.mac_instruction_modal.title', defaultMessage: 'MacOS Installation Guide' })}
                    compassDesign={true}
                >
                    <p style={{ marginBottom: '20px' }}>{formatMessage({ id: 'apps_page.mac_instruction_modal.intro', defaultMessage: 'After installing from the DMG file, you need to perform the following steps:' })}</p>

                    <p><strong>{formatMessage({ id: 'apps_page.mac_instruction_modal.step1', defaultMessage: '# 1. Open Terminal' })}</strong></p>
                    <div style={{ marginBottom: '20px' }}>
                        <img
                            src={macosTerminal}
                            alt={formatMessage({ id: 'apps_page.mac_instruction_modal.image1_alt', defaultMessage: 'Terminal opening guide' })}
                            style={{ maxWidth: '100%', borderRadius: '4px', border: '1px solid var(--center-channel-color-16)' }}
                        />
                    </div>

                    <p><strong>{formatMessage({ id: 'apps_page.mac_instruction_modal.step2', defaultMessage: '# 2. Run the following 2 commands' })}</strong></p>
                    <div style={{ marginBottom: '10px' }}>
                        <img
                            src={macosTerminalCommands}
                            alt={formatMessage({ id: 'apps_page.mac_instruction_modal.image2_alt', defaultMessage: 'Command execution illustration' })}
                            style={{ maxWidth: '100%', borderRadius: '4px', border: '1px solid var(--center-channel-color-16)' }}
                        />
                    </div>
                    <div style={{ background: 'var(--center-channel-bg)', padding: '10px', borderRadius: '4px', fontFamily: 'monospace', marginBottom: '20px', border: '1px solid var(--center-channel-color-16)' }}>
                        xattr -rd com.apple.quarantine /Applications/OKTel.app<br />
                        codesign --force --deep --sign - /Applications/OKTel.app
                    </div>

                    <p><strong>{formatMessage({ id: 'apps_page.mac_instruction_modal.step3', defaultMessage: '# 3. You can then open the application normally' })}</strong></p>
                </GenericModal>
            )}
        </div>
    );
};

export default AppsPage;
