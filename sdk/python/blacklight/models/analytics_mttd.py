from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast


T = TypeVar("T", bound="AnalyticsMttd")


@_attrs_define
class AnalyticsMttd:
    detected_count: int
    undetected_count: int
    unscored_count: int
    unmeasurable_count: int
    attempted_count: int
    blind_filtered: bool
    p50: int | None | Unset = UNSET
    p90: int | None | Unset = UNSET
    max_: int | None | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        detected_count = self.detected_count

        undetected_count = self.undetected_count

        unscored_count = self.unscored_count

        unmeasurable_count = self.unmeasurable_count

        attempted_count = self.attempted_count

        blind_filtered = self.blind_filtered

        p50: int | None | Unset
        if isinstance(self.p50, Unset):
            p50 = UNSET
        else:
            p50 = self.p50

        p90: int | None | Unset
        if isinstance(self.p90, Unset):
            p90 = UNSET
        else:
            p90 = self.p90

        max_: int | None | Unset
        if isinstance(self.max_, Unset):
            max_ = UNSET
        else:
            max_ = self.max_

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "detectedCount": detected_count,
                "undetectedCount": undetected_count,
                "unscoredCount": unscored_count,
                "unmeasurableCount": unmeasurable_count,
                "attemptedCount": attempted_count,
                "blindFiltered": blind_filtered,
            }
        )
        if p50 is not UNSET:
            field_dict["p50"] = p50
        if p90 is not UNSET:
            field_dict["p90"] = p90
        if max_ is not UNSET:
            field_dict["max"] = max_

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        detected_count = d.pop("detectedCount")

        undetected_count = d.pop("undetectedCount")

        unscored_count = d.pop("unscoredCount")

        unmeasurable_count = d.pop("unmeasurableCount")

        attempted_count = d.pop("attemptedCount")

        blind_filtered = d.pop("blindFiltered")

        def _parse_p50(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        p50 = _parse_p50(d.pop("p50", UNSET))

        def _parse_p90(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        p90 = _parse_p90(d.pop("p90", UNSET))

        def _parse_max_(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        max_ = _parse_max_(d.pop("max", UNSET))

        analytics_mttd = cls(
            detected_count=detected_count,
            undetected_count=undetected_count,
            unscored_count=unscored_count,
            unmeasurable_count=unmeasurable_count,
            attempted_count=attempted_count,
            blind_filtered=blind_filtered,
            p50=p50,
            p90=p90,
            max_=max_,
        )

        return analytics_mttd
