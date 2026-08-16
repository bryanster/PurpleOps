from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset


T = TypeVar("T", bound="RecoveryCodeRequest")


@_attrs_define
class RecoveryCodeRequest:
    """Body of `POST /auth/mfa/recovery/verify`: one recovery code, as the
    person has it written down.

    """

    code: str
    """ The code, in any case, with or without the hyphens it was printed
    with. The bounds and the pattern here are a backstop that keeps
    obvious junk off an unauthenticated path — what a code actually is
    gets decided in one place, `internal/authn/recovery.Parse`, and
    anything that gets past this and fails there is refused as an
    incorrect code rather than as a malformed request. Two answers to
    one question is how the two definitions would drift apart.
     """

    def to_dict(self) -> dict[str, Any]:
        code = self.code

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "code": code,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        code = d.pop("code")

        recovery_code_request = cls(
            code=code,
        )

        return recovery_code_request
