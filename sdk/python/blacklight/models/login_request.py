from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="LoginRequest")


@_attrs_define
class LoginRequest:
    """Credentials for `POST /auth/login`."""

    email: str
    """ The address the account was created with, matched without regard to
    case or surrounding whitespace.
     """
    password: str
    """ The password, as typed. The bounds here are a backstop on an
    unauthenticated path, not the policy: the policy lives in one place
    (internal/authn/password) and applies where a password is *set*.
    Holding a login attempt to it would tell an attacker how long the
    password is not.
     """

    def to_dict(self) -> dict[str, Any]:
        email = self.email

        password = self.password

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "email": email,
                "password": password,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        email = d.pop("email")

        password = d.pop("password")

        login_request = cls(
            email=email,
            password=password,
        )

        return login_request
