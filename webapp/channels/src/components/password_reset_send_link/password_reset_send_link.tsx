// Copyright (c) 2015-present Oktel, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, { useCallback, useEffect, useRef, useState } from 'react';
import { defineMessage, FormattedMessage, useIntl } from 'react-intl';
import { useHistory, Link } from 'react-router-dom';

import type { ActionResult } from 'mattermost-redux/types/actions';
import { isEmail } from 'mattermost-redux/utils/helpers';

import Input from 'components/widgets/inputs/input/input';
import type { CustomizeHeaderType } from 'components/header_footer_route/header_footer_route';

import gradientLoginPage from 'images/gradient-login-page.png';
import 'components/login/login.scss';

export interface Props {
    siteName?: string;
    actions: {
        sendPasswordResetEmail: (email: string) => Promise<ActionResult>;
    };
    onCustomizeHeader?: CustomizeHeaderType;
}

const PasswordResetSendLink = ({ siteName, actions, onCustomizeHeader }: Props) => {
    const intl = useIntl();
    const history = useHistory();
    const [error, setError] = useState<React.ReactNode>(null);
    const [updateText, setUpdateText] = useState<React.ReactNode>(null);

    const emailInput = useRef<HTMLInputElement>(null);

    useEffect(() => {
        document.title = intl.formatMessage(
            {
                id: 'password_form.pageTitle',
                defaultMessage: 'Password Reset | {siteName}',
            },
            { siteName: siteName || 'Oktel' },
        );

    }, [intl, siteName]);

    const handleSendLink = async (e: React.FormEvent) => {
        e.preventDefault();

        const email = emailInput.current?.value.trim().toLowerCase();
        if (!email || !isEmail(email)) {
            setError(
                <FormattedMessage
                    id='password_send.error'
                    defaultMessage='Please enter a valid email address.'
                />
            );
            return;
        }

        setError(null);

        const { data, error: sendError } = await actions.sendPasswordResetEmail(email);
        if (data) {
            setError(null);
            setUpdateText(
                <div
                    id='passwordResetEmailSent'
                    className='reset-form alert alert-success'
                >
                    <FormattedMessage
                        id='password_send.link'
                        defaultMessage='If the account exists, a password reset email will be sent to:'
                    />
                    <div>
                        <b>{email}</b>
                    </div>
                    <br />
                    <FormattedMessage
                        id='password_send.checkInbox'
                        defaultMessage='Please check your inbox.'
                    />
                </div>
            );
        } else if (sendError) {
            setError(sendError.message);
            setUpdateText(null);
        }
    };

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
                                id='password_send.title'
                                defaultMessage='Password Reset'
                            />
                        </div>
                        <div className='login-body-card-form'>
                            {updateText || (
                                <form onSubmit={handleSendLink}>
                                    <p>
                                        <FormattedMessage
                                            id='password_send.description'
                                            defaultMessage='To reset your password, enter the email address you used to sign up'
                                        />
                                    </p>
                                    <Input
                                        id='passwordResetEmailInput'
                                        type='email'
                                        name='email'
                                        className='login-body-card-form-input'
                                        label={defineMessage({
                                            id: 'password_send.email',
                                            defaultMessage: 'Email',
                                        })}
                                        placeholder={defineMessage({
                                            id: 'password_send.email.placeholder',
                                            defaultMessage: 'Enter the email address you used to sign up',
                                        })}
                                        ref={emailInput}
                                        spellCheck='false'
                                        autoFocus={true}
                                        hasError={Boolean(error)}
                                        containerClassName={error ? 'has-error' : ''}
                                    />
                                    {error && (
                                        <div className='form-group has-error'>
                                            <label className='control-label'>{error}</label>
                                        </div>
                                    )}
                                    <button
                                        id='passwordResetButton'
                                        type='submit'
                                        className='login-body-card-form-button-submit'
                                    >
                                        <FormattedMessage
                                            id='password_send.reset'
                                            defaultMessage='Reset my password'
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
                            )}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default PasswordResetSendLink;
