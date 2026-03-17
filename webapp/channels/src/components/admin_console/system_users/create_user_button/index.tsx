// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useState} from 'react';
import {FormattedMessage, useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import {GenericModal} from '@mattermost/components';
import type {UserProfile} from '@mattermost/types/users';

import {createUser} from 'mattermost-redux/actions/users';

import Input from 'components/widgets/inputs/input/input';

import './create_user_button.scss';

function generatePassword(): string {
    const lower = 'abcdefghijklmnopqrstuvwxyz';
    const upper = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
    const digits = '0123456789';
    const symbols = '!@#$%^&*';
    const all = lower + upper + digits + symbols;

    // Ensure at least one of each required type
    let password =
        lower[Math.floor(Math.random() * lower.length)] +
        upper[Math.floor(Math.random() * upper.length)] +
        digits[Math.floor(Math.random() * digits.length)] +
        symbols[Math.floor(Math.random() * symbols.length)];

    // Fill remaining length (total 12 chars)
    for (let i = 4; i < 12; i++) {
        password += all[Math.floor(Math.random() * all.length)];
    }

    // Shuffle password
    return password.split('').sort(() => Math.random() - 0.5).join('');
}

type CreatedUserInfo = {
    username: string;
    email: string;
    password: string;
};

type FormState = {
    email: string;
    username: string;
    firstName: string;
    lastName: string;
    password: string;
    showPassword: boolean;
};

export function CreateUserButton() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();

    const [showModal, setShowModal] = useState(false);
    const [isSuccess, setIsSuccess] = useState(false);
    const [createdUser, setCreatedUser] = useState<CreatedUserInfo | null>(null);
    const [copiedField, setCopiedField] = useState<string | null>(null);
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const [form, setForm] = useState<FormState>({
        email: '',
        username: '',
        firstName: '',
        lastName: '',
        password: '',
        showPassword: false,
    });

    const resetState = useCallback(() => {
        setForm({
            email: '',
            username: '',
            firstName: '',
            lastName: '',
            password: '',
            showPassword: false,
        });
        setIsSuccess(false);
        setCreatedUser(null);
        setCopiedField(null);
        setError(null);
        setIsSubmitting(false);
    }, []);

    const handleOpen = useCallback(() => {
        resetState();
        setShowModal(true);
    }, [resetState]);

    const handleClose = useCallback(() => {
        setShowModal(false);
        resetState();
    }, [resetState]);

    const handleGeneratePassword = useCallback(() => {
        setForm((prev) => ({...prev, password: generatePassword(), showPassword: true}));
    }, []);

    const handleCopy = useCallback(async (text: string, fieldId: string) => {
        try {
            await navigator.clipboard.writeText(text);
            setCopiedField(fieldId);
            setTimeout(() => setCopiedField(null), 2000);
        } catch {
            // fallback: select the text
        }
    }, []);

    const handleSubmit = useCallback(async () => {
        setError(null);

        if (!form.email.trim()) {
            setError(formatMessage({id: 'admin.create_user.error.email_required', defaultMessage: 'Email is required.'}));
            return;
        }
        if (!form.username.trim()) {
            setError(formatMessage({id: 'admin.create_user.error.username_required', defaultMessage: 'Username is required.'}));
            return;
        }
        if (!form.password) {
            setError(formatMessage({id: 'admin.create_user.error.password_required', defaultMessage: 'Password is required.'}));
            return;
        }

        setIsSubmitting(true);

        const newUser = {
            email: form.email.trim(),
            username: form.username.trim(),
            first_name: form.firstName.trim(),
            last_name: form.lastName.trim(),
            password: form.password,
        } as UserProfile;

        const result = await dispatch(createUser(newUser, '', '', ''));

        setIsSubmitting(false);

        if ('error' in result && result.error) {
            setError(result.error.message);
            return;
        }

        setCreatedUser({
            username: form.username.trim(),
            email: form.email.trim(),
            password: form.password,
        });
        setIsSuccess(true);
    }, [form, dispatch, formatMessage]);

    const renderForm = () => (
        <div className='CreateUserModal__body'>
            <Input
                type='email'
                name='email'
                label={formatMessage({id: 'admin.create_user.email', defaultMessage: 'Email'})}
                placeholder={formatMessage({id: 'admin.create_user.email_placeholder', defaultMessage: 'Enter email address'})}
                value={form.email}
                onChange={(e) => setForm((prev) => ({...prev, email: e.target.value}))}
                autoFocus={true}
                required={true}
            />
            <Input
                type='text'
                name='username'
                label={formatMessage({id: 'admin.create_user.username', defaultMessage: 'Username'})}
                placeholder={formatMessage({id: 'admin.create_user.username_placeholder', defaultMessage: 'Enter username'})}
                value={form.username}
                onChange={(e) => setForm((prev) => ({...prev, username: e.target.value}))}
                required={true}
            />
            <Input
                type='text'
                name='firstName'
                label={formatMessage({id: 'admin.create_user.first_name', defaultMessage: 'First name (optional)'})}
                placeholder={formatMessage({id: 'admin.create_user.first_name_placeholder', defaultMessage: 'Enter first name'})}
                value={form.firstName}
                onChange={(e) => setForm((prev) => ({...prev, firstName: e.target.value}))}
            />
            <Input
                type='text'
                name='lastName'
                label={formatMessage({id: 'admin.create_user.last_name', defaultMessage: 'Last name (optional)'})}
                placeholder={formatMessage({id: 'admin.create_user.last_name_placeholder', defaultMessage: 'Enter last name'})}
                value={form.lastName}
                onChange={(e) => setForm((prev) => ({...prev, lastName: e.target.value}))}
            />
            <div className='CreateUserModal__password-row'>
                <div className='CreateUserModal__password-input'>
                    <Input
                        type={form.showPassword ? 'text' : 'password'}
                        name='password'
                        autoComplete='new-password'
                        label={formatMessage({id: 'admin.create_user.password', defaultMessage: 'Password'})}
                        placeholder={formatMessage({id: 'admin.create_user.password_placeholder', defaultMessage: 'Enter password'})}
                        value={form.password}
                        onChange={(e) => setForm((prev) => ({...prev, password: e.target.value}))}
                        required={true}
                        inputSuffix={
                            <button
                                type='button'
                                className='btn btn-icon btn-sm CreateUserModal__toggle-password'
                                onClick={() => setForm((prev) => ({...prev, showPassword: !prev.showPassword}))}
                                title={form.showPassword ?
                                    formatMessage({id: 'admin.create_user.hide_password', defaultMessage: 'Hide password'}) :
                                    formatMessage({id: 'admin.create_user.show_password', defaultMessage: 'Show password'})
                                }
                            >
                                <i className={`icon ${form.showPassword ? 'icon-eye-off-outline' : 'icon-eye-outline'}`}/>
                            </button>
                        }
                    />
                </div>
                <button
                    type='button'
                    className='btn btn-tertiary CreateUserModal__generate-btn'
                    onClick={handleGeneratePassword}
                >
                    <FormattedMessage
                        id='admin.create_user.generate_password'
                        defaultMessage='Generate'
                    />
                </button>
            </div>
        </div>
    );

    const renderSuccess = () => (
        <div className='CreateUserModal__success'>
            <div className='CreateUserModal__success-icon'>
                <i className='icon icon-check-circle-outline'/>
            </div>
            <p className='CreateUserModal__success-title'>
                <FormattedMessage
                    id='admin.create_user.success_title'
                    defaultMessage='User created successfully!'
                />
            </p>
            <p className='CreateUserModal__success-desc'>
                <FormattedMessage
                    id='admin.create_user.success_desc'
                    defaultMessage='Share these credentials with the new user. The password will not be shown again.'
                />
            </p>

            <div className='CreateUserModal__credentials'>
                <div className='CreateUserModal__credential-row'>
                    <span className='CreateUserModal__credential-label'>
                        <FormattedMessage id='admin.create_user.cred_username' defaultMessage='Username'/>
                    </span>
                    <div className='CreateUserModal__credential-value-row'>
                        <code className='CreateUserModal__credential-value'>{createdUser?.username}</code>
                        <button
                            type='button'
                            className='btn btn-icon btn-sm CreateUserModal__copy-btn'
                            onClick={() => handleCopy(createdUser?.username ?? '', 'username')}
                            title={formatMessage({id: 'admin.create_user.copy', defaultMessage: 'Copy'})}
                        >
                            <i className={`icon ${copiedField === 'username' ? 'icon-check' : 'icon-content-copy'}`}/>
                        </button>
                    </div>
                </div>

                <div className='CreateUserModal__credential-row'>
                    <span className='CreateUserModal__credential-label'>
                        <FormattedMessage id='admin.create_user.cred_email' defaultMessage='Email'/>
                    </span>
                    <div className='CreateUserModal__credential-value-row'>
                        <code className='CreateUserModal__credential-value'>{createdUser?.email}</code>
                        <button
                            type='button'
                            className='btn btn-icon btn-sm CreateUserModal__copy-btn'
                            onClick={() => handleCopy(createdUser?.email ?? '', 'email')}
                            title={formatMessage({id: 'admin.create_user.copy', defaultMessage: 'Copy'})}
                        >
                            <i className={`icon ${copiedField === 'email' ? 'icon-check' : 'icon-content-copy'}`}/>
                        </button>
                    </div>
                </div>

                <div className='CreateUserModal__credential-row'>
                    <span className='CreateUserModal__credential-label'>
                        <FormattedMessage id='admin.create_user.cred_password' defaultMessage='Password'/>
                    </span>
                    <div className='CreateUserModal__credential-value-row'>
                        <code className='CreateUserModal__credential-value CreateUserModal__credential-password'>{createdUser?.password}</code>
                        <button
                            type='button'
                            className='btn btn-icon btn-sm CreateUserModal__copy-btn'
                            onClick={() => handleCopy(createdUser?.password ?? '', 'password')}
                            title={formatMessage({id: 'admin.create_user.copy', defaultMessage: 'Copy'})}
                        >
                            <i className={`icon ${copiedField === 'password' ? 'icon-check' : 'icon-content-copy'}`}/>
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );

    return (
        <>
            <button
                className='btn btn-primary'
                onClick={handleOpen}
            >
                <i className='icon icon-plus'/>
                <FormattedMessage
                    id='admin.system_users.create_user'
                    defaultMessage='Create user'
                />
            </button>

            {showModal && (
                <GenericModal
                    id='createUserModal'
                    className='CreateUserModal'
                    modalHeaderText={formatMessage({
                        id: 'admin.create_user.title',
                        defaultMessage: 'Create new user',
                    })}
                    show={true}
                    onExited={handleClose}
                    onHide={handleClose}
                    handleCancel={isSuccess ? undefined : handleClose}
                    handleConfirm={isSuccess ? handleClose : handleSubmit}
                    handleEnterKeyPress={isSuccess ? undefined : handleSubmit}
                    confirmButtonText={isSuccess ?
                        formatMessage({id: 'admin.create_user.done', defaultMessage: 'Done'}) :
                        formatMessage({id: 'admin.create_user.create', defaultMessage: 'Create user'})
                    }
                    isConfirmDisabled={isSubmitting}
                    compassDesign={true}
                    autoCloseOnConfirmButton={false}
                    errorText={error ? <span className='error'>{error}</span> : undefined}
                >
                    {isSuccess ? renderSuccess() : renderForm()}
                </GenericModal>
            )}
        </>
    );
}
