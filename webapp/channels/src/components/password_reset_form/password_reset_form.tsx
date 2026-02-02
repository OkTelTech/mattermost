// Copyright (c) 2015-present Oktel, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import React, { useState, memo, useEffect } from 'react';
import { FormattedMessage, useIntl } from 'react-intl';
import { useHistory, useLocation, Link } from 'react-router-dom';

import type { ActionResult } from 'mattermost-redux/types/actions';

import Constants from 'utils/constants';
import type { CustomizeHeaderType } from 'components/header_footer_route/header_footer_route';
import PasswordInput from 'components/widgets/inputs/password_input/password_input';

import gradientLoginPage from 'images/gradient-login-page.png';
import 'components/login/login.scss';

export interface Props {
    actions: {
        resetUserPassword: (token: string, newPassword: string) => Promise<ActionResult>;
    };
    siteName?: string;
    onCustomizeHeader?: CustomizeHeaderType;
}

const PasswordResetForm = ({ siteName, actions, onCustomizeHeader }: Props) => {
    const intl = useIntl();
    const history = useHistory();
    const location = useLocation();

    const [error, setError] = useState<React.ReactNode>(null);
    const [password, setPassword] = useState('');

    useEffect(() => {
        document.title = intl.formatMessage(
            {
                id: 'password_form.pageTitle',
                defaultMessage: 'Password Reset | {siteName}',
            },
            { siteName: siteName || 'Oktel' },
        );

    }, [intl, siteName]);

    const handlePasswordReset = async (e: React.FormEvent) => {
        e.preventDefault();

        const token = (new URLSearchParams(location.search)).get('token');

        if (typeof token !== 'string') {
            setError(
                <FormattedMessage
                    id='password_form.token'
                    defaultMessage='Missing or invalid token'
                />
            );
            return;
        }

        const { data, error: resetError } = await actions.resetUserPassword(token, password);
        if (data) {
            history.push('/login?extra=' + Constants.PASSWORD_CHANGE);
            setError(null);
        } else if (resetError) {
            setError(resetError.message);
        }
    };

    const errorElement = error ? (
        <div className='form-group has-error'>
            <label className='control-label'>
                {error}
            </label>
        </div>
    ) : null;

    return (
        <div className='login-body'>
            <div className='login-body-content'>
                <div className='login-body-message'>
                    <h1 className='login-body-message-title'>{siteName || 'Oktel'}</h1>
                    <div className='login-body-message-subtitle'>
                        <FormattedMessage
                            id='web.root.signup_info'
                            defaultMessage='All your team communication in one place, instantly searchable and accessible anywhere.'
                        />
                    </div>
                    <img
                        className='auth-page-gradient'
                        src={gradientLoginPage}
                        alt='gradient'
                    />
                </div>
                <div className='login-body-card'>
                    <div className='login-body-card-content'>
                        <div className='login-body-card-title'>
                            <FormattedMessage
                                id='password_form.title'
                                defaultMessage='Password Reset'
                            />
                        </div>
                        <form onSubmit={handlePasswordReset} className='login-body-card-form'>
                            <p>
                                <FormattedMessage
                                    id='password_form.enter'
                                    defaultMessage='Enter a new password for your {siteName} account.'
                                    values={{ siteName }}
                                />
                            </p>
                            <PasswordInput
                                className='login-body-card-form-password-input'
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                hasError={Boolean(error)}
                                error={error ? (error as string) : undefined}
                            />
                            {errorElement}
                            <button
                                id='resetPasswordButton'
                                type='submit'
                                className='login-body-card-form-button-submit'
                            >
                                <FormattedMessage
                                    id='password_form.change'
                                    defaultMessage='Change my password'
                                />
                            </button>
                            <div className='login-body-card-form-link'>
                                <Link to='/login'>
                                    <FormattedMessage
                                        id='password_form.back'
                                        defaultMessage='Back to Login'
                                    />
                                </Link>
                            </div>
                        </form>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default memo(PasswordResetForm);
