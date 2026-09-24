// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {act, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import React from 'react';

import {renderWithContext} from 'tests/react_testing_utils';
import * as PwaInstall from 'utils/pwa_install';
import * as UserAgent from 'utils/user_agent';

import MobileInstallButton from './mobile_install_button';

let announcePrompt = () => {};

jest.mock('utils/pwa_install', () => ({
    isIOS: jest.fn(),
    isInAppBrowser: jest.fn(),
    isStandalone: jest.fn(),
    canInstall: jest.fn(),
    subscribeInstall: jest.fn((listener: () => void) => {
        announcePrompt = listener;
        return () => {};
    }),
    promptInstall: jest.fn(async () => true),
}));

jest.mock('utils/user_agent', () => ({
    ...jest.requireActual('utils/user_agent'),
    isAndroid: jest.fn(),
}));

jest.mock('utils/utils', () => ({copyToClipboard: jest.fn()}));

const pwa = jest.mocked(PwaInstall);
const userAgent = jest.mocked(UserAgent);

describe('components/apps_page/MobileInstallButton', () => {
    const originalLocation = window.location;

    beforeEach(() => {
        jest.clearAllMocks();
        pwa.isIOS.mockReturnValue(false);
        pwa.isInAppBrowser.mockReturnValue(false);
        pwa.isStandalone.mockReturnValue(false);
        pwa.canInstall.mockReturnValue(false);
        userAgent.isAndroid.mockReturnValue(false);

        // jsdom cannot navigate, so record where the button sends Safari instead.
        Object.defineProperty(window, 'location', {configurable: true, value: {...originalLocation, href: ''}});
    });

    afterAll(() => {
        Object.defineProperty(window, 'location', {configurable: true, value: originalLocation});
    });

    it('should open the profile in iOS Safari and list the Settings steps', async () => {
        pwa.isIOS.mockReturnValue(true);
        renderWithContext(<MobileInstallButton platform='ios'/>);

        await userEvent.click(screen.getByRole('button', {name: 'Download'}));

        expect(window.location.href).toBe('/static/Oktel.mobileconfig');
        expect(screen.getAllByRole('listitem')).toHaveLength(4);
    });

    it('should show the install dialog on Android when Chrome offered it', async () => {
        userAgent.isAndroid.mockReturnValue(true);
        pwa.canInstall.mockReturnValue(true);
        renderWithContext(<MobileInstallButton platform='android'/>);

        await userEvent.click(screen.getByRole('button', {name: 'Download'}));

        expect(pwa.promptInstall).toHaveBeenCalledTimes(1);
        expect(await screen.findByRole('button', {name: 'Installed on this device'})).toBeDisabled();
    });

    it('should enable Install app on Android once Chrome offers the prompt', async () => {
        userAgent.isAndroid.mockReturnValue(true);
        renderWithContext(<MobileInstallButton platform='android'/>);

        await userEvent.click(screen.getByRole('button', {name: 'Download'}));
        expect(screen.getByRole('button', {name: 'Install app'})).toBeDisabled();

        pwa.canInstall.mockReturnValue(true);
        act(() => announcePrompt());
        await userEvent.click(screen.getByRole('button', {name: 'Install app'}));

        expect(pwa.promptInstall).toHaveBeenCalledTimes(1);
    });

    it('should send an in-app browser on iOS to Safari', async () => {
        pwa.isIOS.mockReturnValue(true);
        pwa.isInAppBrowser.mockReturnValue(true);
        renderWithContext(<MobileInstallButton platform='ios'/>);

        await userEvent.click(screen.getByRole('button', {name: 'Download'}));

        expect(window.location.href).toBe('');
        expect(screen.getByText(/Installing needs Safari/)).toBeInTheDocument();
    });

    it('should only show a QR code on a computer', async () => {
        pwa.canInstall.mockReturnValue(true);
        renderWithContext(<MobileInstallButton platform='android'/>);

        await userEvent.click(screen.getByRole('button', {name: 'Download'}));

        expect(pwa.promptInstall).not.toHaveBeenCalled();
        expect(screen.getByAltText('QR code to the Oktel apps page')).toHaveAttribute('src', '/static/install-qr.svg');
    });

    it('should show the installed state when opened from the Home Screen', () => {
        pwa.isIOS.mockReturnValue(true);
        pwa.isStandalone.mockReturnValue(true);
        renderWithContext(<MobileInstallButton platform='ios'/>);

        expect(screen.getByRole('button', {name: 'Installed on this device'})).toBeDisabled();
    });
});
